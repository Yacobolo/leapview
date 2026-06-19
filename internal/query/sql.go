package query

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Yacobolo/libredash/internal/semantic"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func quoteIdent(value string) (string, error) {
	if !identifierPattern.MatchString(value) {
		return "", fmt.Errorf("invalid identifier %q", value)
	}
	return value, nil
}

func applyAliases(expr string, aliases map[string]tableAlias, fallbackAlias string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return expr
	}
	if identifierPattern.MatchString(expr) {
		return fallbackAlias + "." + expr
	}
	for table, alias := range aliases {
		expr = regexp.MustCompile(`\b`+regexp.QuoteMeta(table)+`\.`).ReplaceAllString(expr, alias.Alias+".")
	}
	expr = strings.ReplaceAll(expr, "{alias}", fallbackAlias)
	return expr
}

func joinSQL(base string, aliases map[string]tableAlias) (string, error) {
	baseIdent, err := quoteIdent(base)
	if err != nil {
		return "", err
	}
	parts := []string{"model." + baseIdent + " t0"}
	for table, alias := range aliases {
		if table == base {
			continue
		}
		if len(alias.Path) != 1 {
			return "", fmt.Errorf("unsupported relationship path to %q", table)
		}
		relationship := alias.Path[0]
		fromTable, fromField, err := splitField(relationship.From)
		if err != nil {
			return "", err
		}
		toTable, toField, err := splitField(relationship.To)
		if err != nil {
			return "", err
		}
		rightIdent, err := quoteIdent(table)
		if err != nil {
			return "", err
		}
		leftAlias := aliases[fromTable].Alias
		rightAlias := aliases[toTable].Alias
		if relationship.Cardinality == "one_to_one" && toTable == base {
			leftAlias = aliases[toTable].Alias
			rightAlias = aliases[fromTable].Alias
			fromField, toField = toField, fromField
		}
		parts = append(parts, fmt.Sprintf("LEFT JOIN model.%s %s ON %s.%s = %s.%s", rightIdent, alias.Alias, leftAlias, fromField, rightAlias, toField))
	}
	return strings.Join(parts, "\n"), nil
}

func dimensionExpr(dimension semantic.MetricDimension, aliases map[string]tableAlias) string {
	alias := aliases[dimension.Table].Alias
	return applyAliases(dimension.SQLExpression(), aliases, alias)
}

func measureExpr(measure semantic.MetricMeasure, aliases map[string]tableAlias) string {
	alias := aliases[measure.Table].Alias
	return applyAliases(measure.Expression, aliases, alias)
}
