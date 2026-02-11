package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseFrames parses Google's length-prefixed frame protocol.
// Format: )]}\n<length>\n<json>
func ParseFrames(data string) ([]json.RawMessage, error) {
	var frames []json.RawMessage
	remaining := data

	for len(remaining) > 0 {
		remaining = strings.TrimSpace(remaining)
		if remaining == "" {
			break
		}

		// Skip the )]}' prefix if present
		if strings.HasPrefix(remaining, ")]}'") {
			remaining = remaining[4:]
			remaining = strings.TrimSpace(remaining)
		}

		// Read the length line
		nlIdx := strings.Index(remaining, "\n")
		if nlIdx < 0 {
			break
		}
		remaining = remaining[nlIdx+1:]

		// Find the JSON array
		jsonStart := strings.Index(remaining, "[")
		if jsonStart < 0 {
			break
		}

		// Parse the JSON, finding the matching bracket
		jsonData, rest, err := extractJSON(remaining[jsonStart:])
		if err != nil {
			break
		}
		frames = append(frames, json.RawMessage(jsonData))
		remaining = rest
	}

	return frames, nil
}

// extractJSON extracts a complete JSON value starting with [ or {
// and returns the JSON string and the remaining data.
func extractJSON(s string) (string, string, error) {
	if len(s) == 0 {
		return "", "", fmt.Errorf("empty input")
	}

	open := s[0]
	var close byte
	switch open {
	case '[':
		close = ']'
	case '{':
		close = '}'
	default:
		return "", "", fmt.Errorf("expected [ or {, got %c", open)
	}

	depth := 0
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == open {
			depth++
		} else if ch == close {
			depth--
			if depth == 0 {
				return s[:i+1], s[i+1:], nil
			}
		}
	}
	return "", "", fmt.Errorf("unbalanced JSON")
}

// NavJSON navigates a nested JSON array by indices.
// e.g., NavJSON(data, 0, 2, 0) accesses data[0][2][0]
func NavJSON(data json.RawMessage, indices ...int) (json.RawMessage, error) {
	current := data
	for _, idx := range indices {
		var arr []json.RawMessage
		if err := json.Unmarshal(current, &arr); err != nil {
			return nil, fmt.Errorf("not an array at index %d: %w", idx, err)
		}
		if idx < 0 || idx >= len(arr) {
			return nil, fmt.Errorf("index %d out of range (len=%d)", idx, len(arr))
		}
		current = arr[idx]
	}
	return current, nil
}
