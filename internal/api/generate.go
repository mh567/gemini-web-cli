package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const streamGenerateURL = "https://gemini.google.com/_/BardChatUi/data/assistant.lamda.BardFrontendService/StreamGenerate"

// ChatSession tracks conversation state.
type ChatSession struct {
	ConversationID string
	ResponseID     string
	ChoiceID       string
}

// ImageInfo represents an image found in the response.
type ImageInfo struct {
	URL       string
	Title     string
	Alt       string
	Generated bool // true = ImageFX generated, false = web image
}

// StreamChunk represents a piece of streamed response.
type StreamChunk struct {
	Text           string
	Thoughts       string
	Images         []ImageInfo
	ConversationID string // session metadata — updated by consumer, not goroutine
	ResponseID     string
	ChoiceID       string
	Done           bool
	Error          error
}

// Candidate represents a single response candidate.
type Candidate struct {
	ChoiceID string
	Text     string
	Thoughts string
	Images   []ImageInfo
}

// FileRef holds an uploaded file's ID and original filename.
type FileRef struct {
	ID   string // upload path returned by UploadFile
	Name string // original filename
}

// buildRequestPayload constructs the request array for StreamGenerate.
func (c *Client) buildRequestPayload(prompt string, session *ChatSession, files []FileRef, gemID string) ([]byte, error) {
	// Build chat metadata
	var metadata interface{}
	if session != nil && session.ConversationID != "" {
		metadata = []interface{}{
			session.ConversationID, session.ResponseID, session.ChoiceID,
			nil, nil, nil, nil, nil, nil, "",
		}
	} else {
		metadata = []interface{}{"", "", "", nil, nil, nil, nil, nil, nil, ""}
	}

	// Build file data: [[url], filename] per file (matches Python library format)
	var fileData interface{}
	if len(files) > 0 {
		refs := make([]interface{}, len(files))
		for i, f := range files {
			refs[i] = []interface{}{[]interface{}{f.ID}, f.Name}
		}
		fileData = refs
	}

	// Always use the 69-element format
	messageContent := []interface{}{prompt, 0, nil, fileData, nil, nil, 0}

	inner := make([]interface{}, 69)
	inner[0] = messageContent
	inner[2] = metadata
	inner[7] = 1 // enable streaming
	if gemID != "" {
		inner[19] = gemID
	}

	return json.Marshal(inner)
}

// StreamGenerate sends a message and returns a channel of streamed chunks.
func (c *Client) StreamGenerate(prompt string, session *ChatSession, files []FileRef) (<-chan StreamChunk, error) {
	return c.StreamGenerateWithGem(prompt, session, files, "")
}

// StreamGenerateWithGem sends a message with an optional Gem ID.
func (c *Client) StreamGenerateWithGem(prompt string, session *ChatSession, files []FileRef, gemID string) (<-chan StreamChunk, error) {
	innerBytes, err := c.buildRequestPayload(prompt, session, files, gemID)
	if err != nil {
		return nil, fmt.Errorf("build payload: %w", err)
	}

	// Double-JSON encode: outer array is [null, "<inner JSON string>"]
	// json.Marshal(string(innerBytes)) will produce a quoted JSON string
	innerStr := string(innerBytes)
	outer := []interface{}{nil, innerStr}
	fReqBytes, err := json.Marshal(outer)
	if err != nil {
		return nil, err
	}
	fReq := string(fReqBytes)

	form := url.Values{}
	form.Set("f.req", fReq)
	form.Set("at", c.snlm0e)

	// Build URL with query parameters
	reqID := c.nextReqID()
	reqURL := fmt.Sprintf("%s?_reqid=%d&rt=c", streamGenerateURL, reqID)
	if c.cfb2h != "" {
		reqURL += "&bl=" + url.QueryEscape(c.cfb2h)
	}
	if c.fdrFJe != "" {
		reqURL += "&f.sid=" + url.QueryEscape(c.fdrFJe)
	}

	req, err := http.NewRequest("POST", reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	c.chromeHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	req.Header.Set("Host", "gemini.google.com")
	req.Header.Set("X-Same-Domain", "1")
	if c.model.HeaderVal != "" {
		req.Header.Set("x-goog-ext-525001261-jspb", c.model.HeaderVal)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	ch := make(chan StreamChunk, 16)
	go c.readStream(resp, ch)
	return ch, nil
}

// lineResult holds a line read from the response body.
type lineResult struct {
	line string
	err  error
}

func (c *Client) readStream(resp *http.Response, ch chan<- StreamChunk) {
	defer close(ch)
	defer resp.Body.Close()

	reader := bufio.NewReaderSize(resp.Body, 256*1024)
	var lastText string
	var lastThoughts string
	var gotAny bool

	// Read lines in a goroutine to allow timeout detection.
	// done channel prevents goroutine leak when readStream exits early.
	lines := make(chan lineResult, 4)
	done := make(chan struct{})
	go func() {
		defer close(lines)
		for {
			line, err := reader.ReadString('\n')
			select {
			case lines <- lineResult{line: line, err: err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	defer close(done)

	// Idle timeout only kicks in AFTER we've received content.
	// Before that, we rely on the HTTP client timeout.
	const idleTimeout = 30 * time.Second
	idleTimer := time.NewTimer(idleTimeout)
	idleTimer.Stop()
	defer idleTimer.Stop()

	for {
		select {
		case lr, ok := <-lines:
			if !ok {
				goto finish
			}
			line := strings.TrimSpace(lr.line)

			if line == "" || strings.HasPrefix(line, ")]}'") || isNumericLine(line) || !strings.HasPrefix(line, "[") {
				if lr.err != nil {
					goto finish
				}
				continue
			}

			var outer []json.RawMessage
			if json.Unmarshal([]byte(line), &outer) != nil {
				if lr.err != nil {
					goto finish
				}
				continue
			}

			for _, item := range outer {
				parsed := c.tryExtractParsed(item)
				if parsed == nil {
					continue
				}
				if parsed.errCode != 0 {
					ch <- StreamChunk{Error: NewGeminiError(parsed.errCode)}
					return
				}
				chunk := StreamChunk{
					ConversationID: parsed.conversationID,
					ResponseID:     parsed.responseID,
					ChoiceID:       parsed.choiceID,
				}
				if parsed.text != "" && parsed.text != lastText {
					chunk.Text = parsed.text
					lastText = parsed.text
				}
				if parsed.thoughts != "" && parsed.thoughts != lastThoughts {
					chunk.Thoughts = parsed.thoughts
					lastThoughts = parsed.thoughts
				}
				if len(parsed.images) > 0 {
					chunk.Images = parsed.images
				}
				if chunk.Text != "" || chunk.Thoughts != "" || len(chunk.Images) > 0 {
					gotAny = true
					ch <- chunk
				}
			}

			if lr.err != nil {
				goto finish
			}
			// Reset idle timer after receiving content
			if gotAny {
				idleTimer.Reset(idleTimeout)
			}

		case <-idleTimer.C:
			goto finish
		}
	}

finish:
	if !gotAny {
		ch <- StreamChunk{Error: fmt.Errorf("failed to parse response")}
		return
	}
	ch <- StreamChunk{Done: true}
}

// isNumericLine checks if a line contains only digits (a length prefix).
func isNumericLine(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// navRaw navigates nested JSON arrays by indices, returning the raw element.
func navRaw(data json.RawMessage, indices ...int) json.RawMessage {
	cur := data
	for _, idx := range indices {
		var arr []json.RawMessage
		if json.Unmarshal(cur, &arr) != nil || idx >= len(arr) {
			return nil
		}
		cur = arr[idx]
	}
	return cur
}

// navInt navigates nested JSON arrays and returns the int value at the path.
func navInt(data json.RawMessage, indices ...int) int {
	raw := navRaw(data, indices...)
	if raw == nil {
		return 0
	}
	var v int
	if json.Unmarshal(raw, &v) != nil {
		return 0
	}
	return v
}

// navString navigates nested JSON arrays and returns the string value at the path.
func navString(data json.RawMessage, indices ...int) string {
	raw := navRaw(data, indices...)
	if raw == nil {
		return ""
	}
	var v string
	if json.Unmarshal(raw, &v) != nil {
		return ""
	}
	return v
}

// parsedResponse holds the full extracted data from a frame.
type parsedResponse struct {
	text           string
	thoughts       string
	images         []ImageInfo
	errCode        int
	candidates     []Candidate
	conversationID string
	responseID     string
	choiceID       string
}

func (c *Client) tryExtractParsed(item json.RawMessage) *parsedResponse {
	var arr []json.RawMessage
	if err := json.Unmarshal(item, &arr); err != nil {
		return nil
	}
	if len(arr) < 3 {
		return nil
	}

	var innerStr string
	if err := json.Unmarshal(arr[2], &innerStr); err != nil {
		return nil
	}
	if innerStr == "" {
		return nil
	}

	var inner []json.RawMessage
	if err := json.Unmarshal([]byte(innerStr), &inner); err != nil {
		return nil
	}

	return c.parseInnerResponse(inner)
}

func (c *Client) parseInnerResponse(inner []json.RawMessage) *parsedResponse {
	resp := &parsedResponse{}

	// Check for error code at inner[5][2][0][1][0]
	if len(inner) > 5 {
		if code := navInt(inner[5], 2, 0, 1, 0); code != 0 {
			resp.errCode = code
			return resp
		}
	}

	// Extract conversation metadata from inner[1]
	if len(inner) > 1 {
		var metaArr []json.RawMessage
		if err := json.Unmarshal(inner[1], &metaArr); err == nil && len(metaArr) >= 2 {
			var convID, respID string
			json.Unmarshal(metaArr[0], &convID)
			json.Unmarshal(metaArr[1], &respID)
			resp.conversationID = convID
			resp.responseID = respID
		}
	}

	if len(inner) < 5 {
		return resp
	}

	var candidates []json.RawMessage
	if err := json.Unmarshal(inner[4], &candidates); err != nil || len(candidates) == 0 {
		return resp
	}

	// Parse all candidates
	for _, rawCand := range candidates {
		var cand []json.RawMessage
		if err := json.Unmarshal(rawCand, &cand); err != nil {
			continue
		}
		candidate := Candidate{
			Text:     c.extractCandidateText(cand),
			Thoughts: c.extractThoughts(cand),
			Images:   c.extractImages(cand),
		}
		if len(cand) > 0 {
			json.Unmarshal(cand[0], &candidate.ChoiceID)
		}
		resp.candidates = append(resp.candidates, candidate)
	}

	// Use first candidate as primary
	if len(resp.candidates) > 0 {
		first := resp.candidates[0]
		resp.text = first.Text
		resp.thoughts = first.Thoughts
		resp.images = first.Images
		resp.choiceID = first.ChoiceID
	}

	return resp
}

// extractCandidateText extracts the main text from a candidate, with HTML unescaping.
// Primary path: candidate[1][0], fallback: candidate[22][0] for googleusercontent URLs.
func (c *Client) extractCandidateText(candidate []json.RawMessage) string {
	if len(candidate) < 2 {
		return ""
	}

	text := navString(candidate[1], 0)

	// If text looks like a googleusercontent card URL, try fallback at [22][0]
	if strings.HasPrefix(text, "http://googleusercontent.com/card_content/") {
		if alt := navString(candidate[22], 0); alt != "" {
			text = alt
		}
	}

	return html.UnescapeString(text)
}

// extractThoughts extracts thinking/reasoning data from candidate[37][0][0].
func (c *Client) extractThoughts(candidate []json.RawMessage) string {
	if len(candidate) <= 37 {
		return ""
	}
	s := navString(candidate[37], 0, 0)
	if s != "" {
		return html.UnescapeString(s)
	}
	return ""
}

// extractImages extracts image information from the candidate data.
func (c *Client) extractImages(candidate []json.RawMessage) []ImageInfo {
	var images []ImageInfo

	if len(candidate) <= 12 {
		return images
	}

	// Web images at candidate[12][1]
	images = append(images, c.parseWebImages(candidate[12])...)

	// Generated images at candidate[12][7][0]
	images = append(images, c.parseGeneratedImages(candidate[12])...)

	return images
}

// parseWebImages extracts web images from candidate[12].
// Path: data[1][*] → URL at [0][0][0], title at [7][0], alt at [0][4].
func (c *Client) parseWebImages(data json.RawMessage) []ImageInfo {
	raw := navRaw(data, 1)
	if raw == nil {
		return nil
	}
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}

	var images []ImageInfo
	for _, item := range items {
		imgURL := navString(item, 0, 0, 0)
		if imgURL == "" {
			continue
		}
		images = append(images, ImageInfo{
			URL:   imgURL,
			Title: navString(item, 7, 0),
			Alt:   navString(item, 0, 4),
		})
	}
	return images
}

// parseGeneratedImages extracts ImageFX generated images from candidate[12].
// Path: data[7][0][*] → URL at [0][3][3].
func (c *Client) parseGeneratedImages(data json.RawMessage) []ImageInfo {
	raw := navRaw(data, 7, 0)
	if raw == nil {
		return nil
	}
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}

	var images []ImageInfo
	for i, item := range items {
		imgURL := navString(item, 0, 3, 3)
		if imgURL == "" {
			continue
		}
		// Replace thumbnail size with full 2048px
		imgURL = strings.Replace(imgURL, "=s512", "=s2048", 1)
		title := fmt.Sprintf("[Generated Image %d]", i+1)
		alt := navString(item, 3, 5, 0)
		images = append(images, ImageInfo{
			URL:       imgURL,
			Title:     title,
			Alt:       alt,
			Generated: true,
		})
	}
	return images
}

// GenerateResult holds the full result of a non-streaming generation.
type GenerateResult struct {
	Text     string
	Thoughts string
	Images   []ImageInfo
}

// Generate sends a message and collects the full response (non-streaming).
func (c *Client) Generate(prompt string, session *ChatSession, files []FileRef) (string, error) {
	res, err := c.GenerateFull(prompt, session, files)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

// GenerateFull sends a message and collects the full response with all metadata.
func (c *Client) GenerateFull(prompt string, session *ChatSession, files []FileRef) (*GenerateResult, error) {
	return c.GenerateFullWithGem(prompt, session, files, "")
}

// GenerateFullWithGem sends a message with an optional Gem ID and collects the full response.
func (c *Client) GenerateFullWithGem(prompt string, session *ChatSession, files []FileRef, gemID string) (*GenerateResult, error) {
	ch, err := c.StreamGenerateWithGem(prompt, session, files, gemID)
	if err != nil {
		return nil, err
	}
	var text, thoughts string
	var images []ImageInfo
	for chunk := range ch {
		if chunk.Error != nil {
			return nil, chunk.Error
		}
		if chunk.Done {
			break
		}
		if chunk.Text != "" {
			text = chunk.Text
		}
		if chunk.Thoughts != "" {
			thoughts = chunk.Thoughts
		}
		if len(chunk.Images) > 0 {
			images = append(images, chunk.Images...)
		}
		// Update session metadata from chunk (safe: same goroutine)
		if session != nil {
			if chunk.ConversationID != "" {
				session.ConversationID = chunk.ConversationID
			}
			if chunk.ResponseID != "" {
				session.ResponseID = chunk.ResponseID
			}
			if chunk.ChoiceID != "" {
				session.ChoiceID = chunk.ChoiceID
			}
		}
	}
	return &GenerateResult{
		Text:     text,
		Thoughts: thoughts,
		Images:   images,
	}, nil
}