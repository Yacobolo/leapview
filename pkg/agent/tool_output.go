package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	toon "github.com/toon-format/toon-go"
)

type ToolOutputFormat string

const (
	ToolOutputTOON ToolOutputFormat = "toon"
	ToolOutputJSON ToolOutputFormat = "json"
)

type ToolOutputConfig struct {
	Format ToolOutputFormat
}

func defaultToolOutputConfig(config ToolOutputConfig) ToolOutputConfig {
	if config.Format == "" {
		config.Format = ToolOutputTOON
	}
	return config
}

func validateToolOutputConfig(config ToolOutputConfig) error {
	switch config.Format {
	case ToolOutputTOON, ToolOutputJSON:
	default:
		return NewError(ErrorCodeInvalidArgument, fmt.Sprintf("unsupported tool output format %q", config.Format), nil)
	}
	return nil
}

func formatToolOutput(value any, config ToolOutputConfig) (string, error) {
	normalized, err := normalizeToolOutput(value)
	if err != nil {
		return "", err
	}
	normalized = wrapTopLevelToolOutput(normalized)
	switch config.Format {
	case ToolOutputJSON:
		body, err := json.Marshal(normalized)
		if err != nil {
			return "", err
		}
		return string(body), nil
	default:
		return toon.MarshalString(normalized)
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
