// Package policy compiles persisted access-policy expressions into the typed,
// immutable form consumed by query authorization.
package policy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	TypeRowFilter  = "row_filter"
	TypeColumnMask = "column_mask"
)

type Compiled struct {
	Type       string
	RowFilter  *RowFilter
	ColumnMask *ColumnMask
	sourceHash [sha256.Size]byte
}

type RowFilter struct {
	AllowAll bool
	Filters  []Filter
}

type ColumnMask struct {
	Fields []string
	Mask   Mask
}

type Filter struct {
	Field    string
	Fact     string
	Operator string
	Values   []any
	Groups   []FilterGroup
	Spatial  *SpatialFilter
}

type FilterGroup struct {
	Filters []Filter
}

type SpatialFilter struct {
	Kind           string
	LatitudeField  string
	LongitudeField string
	Fact           string
	West           float64
	South          float64
	East           float64
	North          float64
	Points         []SpatialPoint
	Center         SpatialPoint
	RadiusMeters   float64
}

type SpatialPoint struct {
	Longitude float64
	Latitude  float64
}

type Mask string

const (
	MaskNull   Mask = "null"
	MaskRedact Mask = "redact"
	MaskZero   Mask = "zero"
)

type expression struct {
	AllowAll bool     `json:"allowAll"`
	Field    string   `json:"field"`
	Columns  []string `json:"columns"`
	Operator string   `json:"operator"`
	Values   []any    `json:"values"`
	Value    any      `json:"value"`
	Filters  []Filter `json:"filters"`
	Mask     string   `json:"mask"`
}

func Compile(id, policyType, expressionJSON string) (Compiled, error) {
	id = strings.TrimSpace(id)
	policyType = strings.TrimSpace(policyType)
	var value expression
	decoder := json.NewDecoder(strings.NewReader(expressionJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return Compiled{}, compileError(id, "expression is invalid: %v", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return Compiled{}, compileError(id, "expression is invalid: %v", err)
	}
	switch policyType {
	case TypeRowFilter:
		row, err := compileRowFilter(id, value)
		if err != nil {
			return Compiled{}, err
		}
		return Compiled{Type: TypeRowFilter, RowFilter: &row, sourceHash: policySourceHash(policyType, expressionJSON)}, nil
	case TypeColumnMask:
		mask, err := compileColumnMask(id, value)
		if err != nil {
			return Compiled{}, err
		}
		return Compiled{Type: TypeColumnMask, ColumnMask: &mask, sourceHash: policySourceHash(policyType, expressionJSON)}, nil
	default:
		return Compiled{}, compileError(id, "has unsupported type %q", policyType)
	}
}

func (compiled Compiled) Matches(policyType, expressionJSON string) bool {
	return compiled.Type == strings.TrimSpace(policyType) && compiled.sourceHash == policySourceHash(strings.TrimSpace(policyType), expressionJSON)
}

func policySourceHash(policyType, expressionJSON string) [sha256.Size]byte {
	return sha256.Sum256([]byte(policyType + "\x00" + expressionJSON))
}

func requireJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("contains multiple JSON values")
}

func compileRowFilter(id string, value expression) (RowFilter, error) {
	hasField := strings.TrimSpace(value.Field) != ""
	hasFilters := len(value.Filters) > 0
	if value.AllowAll {
		if hasField || hasFilters {
			return RowFilter{}, compileError(id, "cannot combine allowAll with field or filters")
		}
		return RowFilter{AllowAll: true}, nil
	}
	if hasField && hasFilters {
		return RowFilter{}, compileError(id, "cannot combine field with filters")
	}
	filters := append([]Filter(nil), value.Filters...)
	if !hasFilters {
		if !hasField {
			return RowFilter{}, compileError(id, "requires field or filters")
		}
		operator := strings.TrimSpace(value.Operator)
		if operator == "" {
			operator = "equals"
		}
		values := append([]any(nil), value.Values...)
		if len(values) == 0 && value.Value != nil {
			values = append(values, value.Value)
		}
		filters = []Filter{{Field: strings.TrimSpace(value.Field), Operator: operator, Values: values}}
	}
	for index := range filters {
		if err := validateFilter(id, fmt.Sprintf("filters[%d]", index), filters[index]); err != nil {
			return RowFilter{}, err
		}
	}
	return RowFilter{Filters: filters}, nil
}

func validateFilter(id, path string, filter Filter) error {
	hasField := strings.TrimSpace(filter.Field) != ""
	hasGroups := len(filter.Groups) > 0
	hasSpatial := filter.Spatial != nil
	forms := 0
	for _, present := range []bool{hasField, hasGroups, hasSpatial} {
		if present {
			forms++
		}
	}
	if forms != 1 {
		return compileError(id, "%s must contain exactly one of field, groups, or spatial", path)
	}
	if hasGroups {
		if filter.Operator != "" || len(filter.Values) != 0 {
			return compileError(id, "%s group cannot contain operator or values", path)
		}
		for groupIndex, group := range filter.Groups {
			if len(group.Filters) == 0 {
				return compileError(id, "%s.groups[%d] requires filters", path, groupIndex)
			}
			for filterIndex, child := range group.Filters {
				if err := validateFilter(id, fmt.Sprintf("%s.groups[%d].filters[%d]", path, groupIndex, filterIndex), child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if hasSpatial {
		if filter.Operator != "" || len(filter.Values) != 0 || strings.TrimSpace(filter.Spatial.LatitudeField) == "" || strings.TrimSpace(filter.Spatial.LongitudeField) == "" {
			return compileError(id, "%s spatial filter is invalid", path)
		}
		switch filter.Spatial.Kind {
		case "box", "lasso", "radius":
			return nil
		default:
			return compileError(id, "%s has unsupported spatial filter kind %q", path, filter.Spatial.Kind)
		}
	}
	return validateScalarFilter(id, path, filter)
}

func validateScalarFilter(id, path string, filter Filter) error {
	operator := strings.TrimSpace(filter.Operator)
	if operator == "" {
		operator = "equals"
	}
	want := 1
	switch operator {
	case "equals", "not_equals", "contains", "not_contains", "starts_with", "ends_with",
		"greater_than", "greater_than_or_equal", "less_than", "less_than_or_equal":
	case "in", "not_in":
		if len(filter.Values) == 0 {
			return compileError(id, "%s %s requires at least one value", path, operator)
		}
		return nil
	case "is_null", "is_not_null":
		want = 0
	default:
		return compileError(id, "%s has unsupported operator %q", path, operator)
	}
	if len(filter.Values) != want {
		return compileError(id, "%s %s requires %d values", path, operator, want)
	}
	return nil
}

func compileColumnMask(id string, value expression) (ColumnMask, error) {
	if value.AllowAll {
		return ColumnMask{}, compileError(id, "column mask cannot use allowAll")
	}
	fields := append([]string(nil), value.Columns...)
	if field := strings.TrimSpace(value.Field); field != "" {
		fields = append(fields, field)
	}
	if len(fields) == 0 {
		return ColumnMask{}, compileError(id, "column mask requires field or columns")
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			return ColumnMask{}, compileError(id, "column mask requires non-empty fields")
		}
		key := strings.ToLower(field)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, field)
	}
	mask, err := compileMask(value.Mask)
	if err != nil {
		return ColumnMask{}, compileError(id, "%v", err)
	}
	return ColumnMask{Fields: normalized, Mask: mask}, nil
}

func compileMask(value string) (Mask, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(MaskNull):
		return MaskNull, nil
	case string(MaskRedact), "redacted":
		return MaskRedact, nil
	case string(MaskZero):
		return MaskZero, nil
	default:
		return "", fmt.Errorf("unsupported column mask %q", value)
	}
}

func compileError(id, format string, args ...any) error {
	return fmt.Errorf("data policy %q %s", id, fmt.Sprintf(format, args...))
}
