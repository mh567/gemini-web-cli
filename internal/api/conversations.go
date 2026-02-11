package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const batchExecuteURL = "https://gemini.google.com/_/BardChatUi/data/batchexecute"

// jsonStr returns a JSON-encoded string (with quotes stripped) safe for embedding in payloads.
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

// Conversation represents a chat conversation.
type Conversation struct {
	ID    string
	Title string
}

// Message represents a single message in a conversation.
type Message struct {
	Role string // "user" or "model"
	Text string
}

// ConversationDetail holds messages plus session metadata for continuation.
type ConversationDetail struct {
	Messages   []Message
	ResponseID string
	ChoiceID   string
}

// batchExecute performs a batchexecute RPC call.
func (c *Client) batchExecute(rpcID, payload string) (string, error) {
	reqID := c.nextReqID()

	// Use json.Marshal to properly escape the payload string (which contains quotes)
	fReqData := []interface{}{[]interface{}{[]interface{}{rpcID, payload, nil, "generic"}}}
	fReqBytes, err := json.Marshal(fReqData)
	if err != nil {
		return "", fmt.Errorf("marshal f.req: %w", err)
	}
	fReq := string(fReqBytes)
	form := url.Values{}
	form.Set("f.req", fReq)
	form.Set("at", c.snlm0e)

	reqURL := fmt.Sprintf("%s?rpcids=%s&source-path=%%2Fapp&_reqid=%d&rt=c",
		batchExecuteURL, rpcID, reqID)
	if c.cfb2h != "" {
		reqURL += "&bl=" + url.QueryEscape(c.cfb2h)
	}
	if c.fdrFJe != "" {
		reqURL += "&f.sid=" + url.QueryEscape(c.fdrFJe)
	}

	req, err := http.NewRequest("POST", reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	c.chromeHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Host", "gemini.google.com")
	req.Header.Set("X-Same-Domain", "1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("batchexecute HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ListConversations retrieves the list of conversations.
func (c *Client) ListConversations() ([]Conversation, error) {
	// Payload: [null, null, count] — request up to 50 conversations
	raw, err := c.batchExecute("MaZiqc", "[]")
	if err != nil {
		return nil, fmt.Errorf("list conversations failed: %w", err)
	}

	frames, err := ParseFrames(raw)
	if err != nil || len(frames) == 0 {
		return nil, fmt.Errorf("failed to parse conversation list (no frames)")
	}

	return parseConversationList(frames)
}

func parseConversationList(frames []json.RawMessage) ([]Conversation, error) {
	var convs []Conversation
	var lastErr error

	for _, frame := range frames {
		var outer []json.RawMessage
		if err := json.Unmarshal(frame, &outer); err != nil {
			lastErr = fmt.Errorf("frame unmarshal: %w", err)
			continue
		}
		for _, item := range outer {
			var arr []json.RawMessage
			if err := json.Unmarshal(item, &arr); err != nil {
				continue
			}
			if len(arr) < 3 {
				continue
			}

			// Check RPC ID matches (arr[0] = "wrb.fr", arr[1] = rpcID)
			var rpcID string
			if len(arr) > 1 {
				json.Unmarshal(arr[1], &rpcID)
			}
			if rpcID != "MaZiqc" {
				continue
			}

			// arr[2] contains the inner JSON string
			var innerStr string
			if err := json.Unmarshal(arr[2], &innerStr); err != nil {
				lastErr = fmt.Errorf("inner string unmarshal: %w", err)
				continue
			}
			if innerStr == "" {
				continue
			}

			var inner []json.RawMessage
			if err := json.Unmarshal([]byte(innerStr), &inner); err != nil {
				lastErr = fmt.Errorf("inner JSON parse: %w", err)
				continue
			}

			convs = append(convs, extractConversations(inner)...)
		}
	}

	if len(convs) == 0 && lastErr != nil {
		return nil, fmt.Errorf("no conversations parsed: %w", lastErr)
	}
	return convs, nil
}

// extractConversations tries multiple known response structures.
func extractConversations(inner []json.RawMessage) []Conversation {
	var convs []Conversation

	if len(inner) == 0 {
		return nil
	}

	// Response format: [null, null, [[conv1], [conv2], ...]]
	// Try inner[2] first, then fall back to inner[0]
	var entries []json.RawMessage
	for _, idx := range []int{2, 0} {
		if idx >= len(inner) {
			continue
		}
		if err := json.Unmarshal(inner[idx], &entries); err == nil && len(entries) > 0 {
			break
		}
		entries = nil
	}
	if entries == nil {
		return nil
	}

	for _, entry := range entries {
		var e []json.RawMessage
		if err := json.Unmarshal(entry, &e); err != nil {
			continue
		}
		if len(e) < 2 {
			continue
		}

		var id, title string
		json.Unmarshal(e[0], &id)
		json.Unmarshal(e[1], &title)
		if id != "" {
			if title == "" {
				title = "(untitled)"
			}
			convs = append(convs, Conversation{ID: id, Title: title})
		}
	}

	return convs
}

// GetConversation retrieves messages from a specific conversation.
func (c *Client) GetConversation(convID string) ([]Message, error) {
	payload := fmt.Sprintf(`["%s"]`, jsonStr(convID))
	raw, err := c.batchExecute("hNvQHb", payload)
	if err != nil {
		return nil, fmt.Errorf("get conversation failed: %w", err)
	}

	frames, err := ParseFrames(raw)
	if err != nil || len(frames) == 0 {
		return nil, fmt.Errorf("failed to parse conversation")
	}

	return parseMessages(frames)
}

func parseMessages(frames []json.RawMessage) ([]Message, error) {
	var msgs []Message
	for _, frame := range frames {
		var outer []json.RawMessage
		if err := json.Unmarshal(frame, &outer); err != nil {
			continue
		}
		for _, item := range outer {
			var arr []json.RawMessage
			if err := json.Unmarshal(item, &arr); err != nil {
				continue
			}
			if len(arr) < 3 {
				continue
			}
			var innerStr string
			if err := json.Unmarshal(arr[2], &innerStr); err != nil {
				continue
			}
			var inner []json.RawMessage
			if err := json.Unmarshal([]byte(innerStr), &inner); err != nil {
				continue
			}
			// Parse message pairs from inner data
			if len(inner) > 0 {
				var turns []json.RawMessage
				if err := json.Unmarshal(inner[0], &turns); err == nil {
					for _, turn := range turns {
						extractTurnMessages(turn, &msgs)
					}
				}
			}
		}
	}
	return msgs, nil
}

// GetConversationDetail retrieves messages and session metadata for continuation.
func (c *Client) GetConversationDetail(convID string) (*ConversationDetail, error) {
	payload := fmt.Sprintf(`["%s"]`, jsonStr(convID))
	raw, err := c.batchExecute("hNvQHb", payload)
	if err != nil {
		return nil, fmt.Errorf("get conversation failed: %w", err)
	}

	frames, err := ParseFrames(raw)
	if err != nil || len(frames) == 0 {
		return nil, fmt.Errorf("failed to parse conversation")
	}

	return parseConversationDetail(frames)
}

func parseConversationDetail(frames []json.RawMessage) (*ConversationDetail, error) {
	detail := &ConversationDetail{}
	for _, frame := range frames {
		var outer []json.RawMessage
		if err := json.Unmarshal(frame, &outer); err != nil {
			continue
		}
		for _, item := range outer {
			var arr []json.RawMessage
			if err := json.Unmarshal(item, &arr); err != nil {
				continue
			}
			if len(arr) < 3 {
				continue
			}
			var innerStr string
			if err := json.Unmarshal(arr[2], &innerStr); err != nil {
				continue
			}
			var inner []json.RawMessage
			if err := json.Unmarshal([]byte(innerStr), &inner); err != nil {
				continue
			}

			// Extract metadata from the last turn in inner[0]:
			// turn[0][1] = responseID, turn[3][3] = choiceID
			if len(inner) > 0 {
				var turns []json.RawMessage
				if err := json.Unmarshal(inner[0], &turns); err == nil {
					for _, turn := range turns {
						extractTurnMessages(turn, &detail.Messages)
					}
					// Get metadata from the last turn for continuation
					if len(turns) > 0 {
						lastTurn := turns[len(turns)-1]
						var lt []json.RawMessage
						if json.Unmarshal(lastTurn, &lt) == nil {
							// ResponseID at turn[0][1]
							if len(lt) > 0 {
								if raw, err := NavJSON(lt[0], 1); err == nil {
									json.Unmarshal(raw, &detail.ResponseID)
								}
							}
							// ChoiceID at turn[3][3]
							if len(lt) > 3 {
								if raw, err := NavJSON(lt[3], 3); err == nil {
									json.Unmarshal(raw, &detail.ChoiceID)
								}
							}
						}
					}
				}
			}
		}
	}
	return detail, nil
}

func extractTurnMessages(turn json.RawMessage, msgs *[]Message) {
	var t []json.RawMessage
	if err := json.Unmarshal(turn, &t); err != nil {
		return
	}
	// Turn structure: [0]=[convID,responseID], [1]=null, [2]=userMsg, [3]=modelResp, [4]=timestamp
	// User text at turn[2][0][0]
	if len(t) > 2 {
		var userText string
		if raw, err := NavJSON(t[2], 0, 0); err == nil {
			json.Unmarshal(raw, &userText)
		}
		if userText != "" {
			*msgs = append(*msgs, Message{Role: "user", Text: userText})
		}
	}
	// Model text at turn[3][0][0][1][0]
	if len(t) > 3 {
		var modelText string
		if raw, err := NavJSON(t[3], 0, 0, 1, 0); err == nil {
			json.Unmarshal(raw, &modelText)
		}
		if modelText != "" {
			*msgs = append(*msgs, Message{Role: "model", Text: modelText})
		}
	}
}

// DeleteConversation deletes a conversation by ID.
func (c *Client) DeleteConversation(convID string) error {
	payload := fmt.Sprintf(`["%s"]`, jsonStr(convID))
	_, err := c.batchExecute("GzXR5e", payload)
	return err
}
