package api

import (
	"encoding/json"
	"fmt"
)

// Gem represents a Gemini Gem (system prompt preset).
type Gem struct {
	ID          string
	Name        string
	Description string
	Prompt      string
	Predefined  bool
}

// ListGems retrieves all available Gems via batchexecute RPC.
func (c *Client) ListGems() ([]Gem, error) {
	raw, err := c.batchExecute("CNgdBe", "[]")
	if err != nil {
		return nil, fmt.Errorf("list gems failed: %w", err)
	}

	frames, err := ParseFrames(raw)
	if err != nil || len(frames) == 0 {
		return nil, fmt.Errorf("failed to parse gems list")
	}

	return parseGemsList(frames)
}

func parseGemsList(frames []json.RawMessage) ([]Gem, error) {
	var gems []Gem
	for _, frame := range frames {
		var outer []json.RawMessage
		if err := json.Unmarshal(frame, &outer); err != nil {
			continue
		}
		for _, item := range outer {
			var arr []json.RawMessage
			if err := json.Unmarshal(item, &arr); err != nil || len(arr) < 3 {
				continue
			}
			var innerStr string
			if err := json.Unmarshal(arr[2], &innerStr); err != nil || innerStr == "" {
				continue
			}
			var inner []json.RawMessage
			if err := json.Unmarshal([]byte(innerStr), &inner); err != nil || len(inner) == 0 {
				continue
			}
			var entries []json.RawMessage
			if err := json.Unmarshal(inner[0], &entries); err != nil {
				continue
			}
			for _, entry := range entries {
				if g := parseGemEntry(entry); g != nil {
					gems = append(gems, *g)
				}
			}
		}
	}
	return gems, nil
}

func parseGemEntry(entry json.RawMessage) *Gem {
	var e []json.RawMessage
	if err := json.Unmarshal(entry, &e); err != nil || len(e) < 3 {
		return nil
	}
	var id, name string
	json.Unmarshal(e[0], &id)
	json.Unmarshal(e[1], &name)
	if id == "" {
		return nil
	}
	g := &Gem{ID: id, Name: name}
	if len(e) > 2 {
		json.Unmarshal(e[2], &g.Description)
	}
	if len(e) > 3 {
		json.Unmarshal(e[3], &g.Prompt)
	}
	if len(e) > 4 {
		json.Unmarshal(e[4], &g.Predefined)
	}
	return g
}

// CreateGem creates a new Gem with the given name, prompt, and description.
func (c *Client) CreateGem(name, prompt, desc string) (string, error) {
	payload := fmt.Sprintf(
		`[null,"%s","%s","%s"]`,
		jsonStr(name),
		jsonStr(prompt),
		jsonStr(desc),
	)
	raw, err := c.batchExecute("oMH3Zd", payload)
	if err != nil {
		return "", fmt.Errorf("create gem failed: %w", err)
	}
	// Try to extract the new gem ID from the response
	frames, err := ParseFrames(raw)
	if err != nil || len(frames) == 0 {
		return "", fmt.Errorf("failed to parse create gem response")
	}
	id := extractGemID(frames)
	if id == "" {
		return "", fmt.Errorf("gem created but could not extract ID")
	}
	return id, nil
}

func extractGemID(frames []json.RawMessage) string {
	for _, frame := range frames {
		var outer []json.RawMessage
		if err := json.Unmarshal(frame, &outer); err != nil {
			continue
		}
		for _, item := range outer {
			var arr []json.RawMessage
			if err := json.Unmarshal(item, &arr); err != nil || len(arr) < 3 {
				continue
			}
			var innerStr string
			if err := json.Unmarshal(arr[2], &innerStr); err != nil || innerStr == "" {
				continue
			}
			var inner []json.RawMessage
			if err := json.Unmarshal([]byte(innerStr), &inner); err != nil || len(inner) == 0 {
				continue
			}
			var id string
			if json.Unmarshal(inner[0], &id) == nil && id != "" {
				return id
			}
		}
	}
	return ""
}

// UpdateGem updates an existing Gem.
func (c *Client) UpdateGem(id, name, prompt, desc string) error {
	payload := fmt.Sprintf(
		`["%s","%s","%s","%s"]`,
		jsonStr(id),
		jsonStr(name),
		jsonStr(prompt),
		jsonStr(desc),
	)
	_, err := c.batchExecute("kHv0Vd", payload)
	if err != nil {
		return fmt.Errorf("update gem failed: %w", err)
	}
	return nil
}

// DeleteGem deletes a Gem by ID.
func (c *Client) DeleteGem(id string) error {
	payload := fmt.Sprintf(`["%s"]`, jsonStr(id))
	_, err := c.batchExecute("UXcSJb", payload)
	if err != nil {
		return fmt.Errorf("delete gem failed: %w", err)
	}
	return nil
}