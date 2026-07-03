package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type ToolOutputFormat string

const (
	ToolOutputTOON ToolOutputFormat = "toon"
	ToolOutputJSON ToolOutputFormat = "json"
)

type ToolOutputConfig struct {
	Format         ToolOutputFormat
	MaxStringChars int
	MaxArrayItems  int
	MaxObjectDepth int
}

type toolOutputTruncation struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Shown      int    `json:"shown,omitempty"`
	Total      int    `json:"total,omitempty"`
	ShownChars int    `json:"shown_chars,omitempty"`
	TotalChars int    `json:"total_chars,omitempty"`
	MaxDepth   int    `json:"max_depth,omitempty"`
}

func defaultToolOutputConfig(config ToolOutputConfig) ToolOutputConfig {
	if config.Format == "" {
		config.Format = ToolOutputTOON
	}
	if config.MaxStringChars == 0 {
		config.MaxStringChars = 2000
	}
	if config.MaxArrayItems == 0 {
		config.MaxArrayItems = 50
	}
	if config.MaxObjectDepth == 0 {
		config.MaxObjectDepth = 8
	}
	return config
}

func validateToolOutputConfig(config ToolOutputConfig) error {
	switch config.Format {
	case ToolOutputTOON, ToolOutputJSON:
	default:
		return NewError(ErrorCodeInvalidArgument, fmt.Sprintf("unsupported tool output format %q", config.Format), nil)
	}
	if config.MaxStringChars <= 0 {
		return NewError(ErrorCodeInvalidArgument, "max tool output string chars must be positive", nil)
	}
	if config.MaxArrayItems <= 0 {
		return NewError(ErrorCodeInvalidArgument, "max tool output array items must be positive", nil)
	}
	if config.MaxObjectDepth <= 0 {
		return NewError(ErrorCodeInvalidArgument, "max tool output object depth must be positive", nil)
	}
	return nil
}

func formatToolOutput(value any, config ToolOutputConfig) (string, error) {
	normalized, err := normalizeToolOutput(value)
	if err != nil {
		return "", err
	}
	normalized = wrapTopLevelToolOutput(normalized)
	var truncations []toolOutputTruncation
	truncated := truncateToolOutput(normalized, "$", 0, config, &truncations)
	if len(truncations) > 0 {
		if object, ok := truncated.(map[string]any); ok {
			object["_meta"] = map[string]any{
				"truncated":   true,
				"truncations": truncationsAsValues(truncations),
			}
		}
	}
	switch config.Format {
	case ToolOutputJSON:
		body, err := json.Marshal(truncated)
		if err != nil {
			return "", err
		}
		return string(body), nil
	default:
		return serializeTOON(truncated), nil
	}
}

func normalizeToolOutput(value any) (any, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var out any
	if err := decoder.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func wrapTopLevelToolOutput(value any) any {
	switch typed := value.(type) {
	case []any:
		return map[string]any{"count": json.Number(strconv.Itoa(len(typed))), "items": typed}
	case map[string]any:
		return typed
	default:
		return map[string]any{"value": typed}
	}
}

func truncateToolOutput(value any, path string, depth int, config ToolOutputConfig, truncations *[]toolOutputTruncation) any {
	if depth >= config.MaxObjectDepth {
		*truncations = append(*truncations, toolOutputTruncation{Path: path, Kind: "depth", MaxDepth: config.MaxObjectDepth})
		return fmt.Sprintf("[truncated: max depth %d]", config.MaxObjectDepth)
	}
	switch typed := value.(type) {
	case string:
		runes := []rune(typed)
		if len(runes) <= config.MaxStringChars {
			return typed
		}
		*truncations = append(*truncations, toolOutputTruncation{Path: path, Kind: "string", ShownChars: config.MaxStringChars, TotalChars: len(runes)})
		return string(runes[:config.MaxStringChars])
	case []any:
		total := len(typed)
		shown := total
		if shown > config.MaxArrayItems {
			shown = config.MaxArrayItems
			*truncations = append(*truncations, toolOutputTruncation{Path: path, Kind: "array", Shown: shown, Total: total})
		}
		out := make([]any, shown)
		for i := 0; i < shown; i++ {
			out[i] = truncateToolOutput(typed[i], fmt.Sprintf("%s[%d]", path, i), depth+1, config, truncations)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for _, key := range sortedToolOutputKeys(typed) {
			out[key] = truncateToolOutput(typed[key], path+"."+key, depth+1, config, truncations)
		}
		return out
	default:
		return typed
	}
}

func truncationsAsValues(truncations []toolOutputTruncation) []any {
	out := make([]any, len(truncations))
	for i, truncation := range truncations {
		item := map[string]any{
			"kind": truncation.Kind,
			"path": truncation.Path,
		}
		if truncation.Shown > 0 || truncation.Total > 0 {
			item["shown"] = truncation.Shown
			item["total"] = truncation.Total
		}
		if truncation.ShownChars > 0 || truncation.TotalChars > 0 {
			item["shown_chars"] = truncation.ShownChars
			item["total_chars"] = truncation.TotalChars
		}
		if truncation.MaxDepth > 0 {
			item["max_depth"] = truncation.MaxDepth
		}
		out[i] = item
	}
	return out
}

func serializeTOON(value any) string {
	var b strings.Builder
	writeTOONValue(&b, "", value, 0, true)
	return strings.TrimRight(b.String(), "\n")
}

func writeTOONValue(b *strings.Builder, key string, value any, indent int, root bool) {
	prefix := strings.Repeat("  ", indent)
	switch typed := value.(type) {
	case map[string]any:
		if !root && key != "" {
			b.WriteString(prefix)
			b.WriteString(key)
			b.WriteString(":\n")
			indent++
			prefix = strings.Repeat("  ", indent)
		}
		for _, childKey := range sortedToolOutputKeys(typed) {
			writeTOONValue(b, childKey, typed[childKey], indent, false)
		}
	case []any:
		writeTOONArray(b, key, typed, indent)
	default:
		b.WriteString(prefix)
		if key != "" {
			b.WriteString(key)
			b.WriteString(": ")
		}
		b.WriteString(toTOONScalar(typed))
		b.WriteByte('\n')
	}
}

func writeTOONArray(b *strings.Builder, key string, values []any, indent int) {
	prefix := strings.Repeat("  ", indent)
	if len(values) == 0 {
		b.WriteString(prefix)
		b.WriteString(key)
		b.WriteString("[0]:\n")
		return
	}
	fields, objects := toonArrayFields(values)
	if objects {
		b.WriteString(prefix)
		b.WriteString(key)
		b.WriteString("[")
		b.WriteString(strconv.Itoa(len(values)))
		b.WriteString("]{")
		b.WriteString(strings.Join(fields, ","))
		b.WriteString("}:\n")
		for _, value := range values {
			object := value.(map[string]any)
			b.WriteString(prefix)
			b.WriteString("  ")
			for i, field := range fields {
				if i > 0 {
					b.WriteByte(',')
				}
				b.WriteString(toTOONScalar(object[field]))
			}
			b.WriteByte('\n')
		}
		return
	}
	b.WriteString(prefix)
	b.WriteString(key)
	b.WriteString("[")
	b.WriteString(strconv.Itoa(len(values)))
	b.WriteString("]:\n")
	for _, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			b.WriteString(prefix)
			b.WriteString("  -\n")
			for _, childKey := range sortedToolOutputKeys(typed) {
				writeTOONValue(b, childKey, typed[childKey], indent+2, false)
			}
		case []any:
			b.WriteString(prefix)
			b.WriteString("  -\n")
			writeTOONArray(b, "items", typed, indent+2)
		default:
			b.WriteString(prefix)
			b.WriteString("  ")
			b.WriteString(toTOONScalar(value))
			b.WriteByte('\n')
		}
	}
}

func toonArrayFields(values []any) ([]string, bool) {
	var fields []string
	for i, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		fieldSet := map[string]struct{}{}
		for key := range object {
			if _, nestedObject := object[key].(map[string]any); nestedObject {
				return nil, false
			}
			if _, nestedArray := object[key].([]any); nestedArray {
				return nil, false
			}
			fieldSet[key] = struct{}{}
		}
		current := make([]string, 0, len(fieldSet))
		for field := range fieldSet {
			current = append(current, field)
		}
		sort.Strings(current)
		if i == 0 {
			fields = current
			continue
		}
		if len(current) != len(fields) {
			return nil, false
		}
		for j := range current {
			if current[j] != fields[j] {
				return nil, false
			}
		}
	}
	return fields, true
}

func toTOONScalar(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return quoteTOONString(typed)
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return quoteTOONString(fmt.Sprint(typed))
	}
}

func quoteTOONString(value string) string {
	if value == "" {
		return strconv.Quote(value)
	}
	for _, r := range value {
		if r == ',' || r == ':' || r == '\n' || r == '\r' || r == '[' || r == ']' || r == '{' || r == '}' || unicode.IsControl(r) {
			return strconv.Quote(value)
		}
	}
	if strings.TrimSpace(value) != value {
		return strconv.Quote(value)
	}
	return value
}

func sortedToolOutputKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
