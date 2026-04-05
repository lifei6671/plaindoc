package service

import (
	"encoding/json"
	"strings"
)

// DecodeThemeStyleJSON 解析主题样式 JSON，并兼容历史 MySQL 种子里的 fontFamily 脏数据。
func DecodeThemeStyleJSON(rawValue string) (map[string]any, error) {
	if strings.TrimSpace(rawValue) == "" {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(rawValue), &payload); err == nil {
		if payload == nil {
			return map[string]any{}, nil
		}
		return payload, nil
	}

	repaired, repairedOK := repairLegacyThemeStyleJSON(rawValue)
	if !repairedOK {
		var decodeErr error
		if err := json.Unmarshal([]byte(rawValue), &payload); err != nil {
			decodeErr = err
		}
		return nil, decodeErr
	}

	if err := json.Unmarshal([]byte(repaired), &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return map[string]any{}, nil
	}
	return payload, nil
}

func repairLegacyThemeStyleJSON(rawValue string) (string, bool) {
	const key = `"fontFamily":`

	normalized := strings.TrimSpace(rawValue)
	keyIndex := strings.Index(normalized, key)
	if keyIndex < 0 {
		return "", false
	}

	valueStart := keyIndex + len(key)
	valueEnd := findLegacyThemeNextPropertyDelimiter(normalized, valueStart)
	if valueEnd < 0 {
		trimmedRight := strings.TrimRight(normalized, " \t\r\n")
		if !strings.HasSuffix(trimmedRight, "}") {
			return "", false
		}
		valueEnd = strings.LastIndex(trimmedRight, "}")
	}

	fontFamily := normalizeLegacyThemeFontFamily(normalized[valueStart:valueEnd])
	if fontFamily == "" {
		return "", false
	}

	encodedFontFamily, err := json.Marshal(fontFamily)
	if err != nil {
		return "", false
	}

	var builder strings.Builder
	builder.Grow(len(normalized) + len(encodedFontFamily))
	builder.WriteString(normalized[:valueStart])
	builder.Write(encodedFontFamily)
	builder.WriteString(normalized[valueEnd:])
	return builder.String(), true
}

func findLegacyThemeNextPropertyDelimiter(rawValue string, start int) int {
	for idx := start; idx < len(rawValue); idx++ {
		if rawValue[idx] != ',' {
			continue
		}

		cursor := idx + 1
		for cursor < len(rawValue) && (rawValue[cursor] == ' ' || rawValue[cursor] == '\t' || rawValue[cursor] == '\n' || rawValue[cursor] == '\r') {
			cursor++
		}
		if cursor >= len(rawValue) || rawValue[cursor] != '"' {
			continue
		}

		propertyEnd := strings.IndexByte(rawValue[cursor+1:], '"')
		if propertyEnd < 0 {
			continue
		}
		propertyEnd += cursor + 1

		cursor = propertyEnd + 1
		for cursor < len(rawValue) && (rawValue[cursor] == ' ' || rawValue[cursor] == '\t' || rawValue[cursor] == '\n' || rawValue[cursor] == '\r') {
			cursor++
		}
		if cursor < len(rawValue) && rawValue[cursor] == ':' {
			return idx
		}
	}
	return -1
}

func normalizeLegacyThemeFontFamily(rawValue string) string {
	value := strings.TrimSpace(rawValue)
	value = strings.TrimPrefix(value, `"`)
	value = strings.TrimSuffix(value, `"`)
	value = strings.ReplaceAll(value, `\"`, `"`)
	value = strings.ReplaceAll(value, `"`, "")

	segments := strings.Split(value, ",")
	cleaned := make([]string, 0, len(segments))
	for _, segment := range segments {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return strings.Join(cleaned, ", ")
}
