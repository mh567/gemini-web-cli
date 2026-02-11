package api

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

const uploadURL = "https://content-push.googleapis.com/upload"

// mimeTypes maps file extensions to MIME types.
var mimeTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".pdf":  "application/pdf",
	".txt":  "text/plain",
	".csv":  "text/csv",
}

func detectMIME(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if mt, ok := mimeTypes[ext]; ok {
		return mt
	}
	return "application/octet-stream"
}

// UploadFile uploads a file and returns a FileRef for use in generation requests.
func (c *Client) UploadFile(filePath string) (FileRef, error) {
	// Notify Gemini that file activity is happening (matches Python library behavior)
	c.batchExecute("ESY5D", `[[["bard_activity_enabled"]]]`)

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer writer.Close()
		defer pw.Close()

		// Open file for streaming (not reading into memory)
		file, err := os.Open(filePath)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("cannot open file: %w", err))
			return
		}
		defer file.Close()

		// Create a form part with the correct MIME type
		mimeType := detectMIME(filePath)
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filepath.Base(filePath)))
		h.Set("Content-Type", mimeType)
		part, err := writer.CreatePart(h)
		if err != nil {
			pw.CloseWithError(err)
			return
		}

		// Stream file content directly without buffering in memory
		if _, err := io.Copy(part, file); err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	fileName := filepath.Base(filePath)

	req, err := http.NewRequest("POST", uploadURL, pr)
	if err != nil {
		pr.Close()
		return FileRef{}, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Push-ID", "feeds/mcudyrk2a4khkz")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return FileRef{}, fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return FileRef{}, fmt.Errorf("upload HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FileRef{}, fmt.Errorf("failed to read upload response: %w", err)
	}

	result := strings.TrimSpace(string(body))
	if result == "" {
		return FileRef{}, fmt.Errorf("empty upload response")
	}

	// Try JSON first (some responses are JSON with upload_id)
	var jsonResult map[string]interface{}
	if json.Unmarshal(body, &jsonResult) == nil {
		if id, ok := jsonResult["upload_id"].(string); ok {
			return FileRef{ID: id, Name: fileName}, nil
		}
	}

	// Otherwise the response body itself is the file identifier
	return FileRef{ID: result, Name: fileName}, nil
}
