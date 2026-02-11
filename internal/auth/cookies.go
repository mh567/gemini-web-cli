package auth

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
)

const (
	geminiAppURL    = "https://gemini.google.com/app"
	rotateCookieURL = "https://accounts.google.com/RotateCookies"
)

// BuildCookieHeader creates the Cookie header string from AccountCookies.
func BuildCookieHeader(c *AccountCookies) string {
	parts := []string{
		"__Secure-1PSID=" + c.PSID,
		"__Secure-1PSIDTS=" + c.PSIDTS,
		"__Secure-1PSIDCC=" + c.PSIDCC,
	}
	if c.NID != "" {
		parts = append(parts, "NID="+c.NID)
	}
	return strings.Join(parts, "; ")
}

// ValidateCookies checks if the cookies are still valid by scanning the Gemini page
// for the SNlM0e token, without reading the entire response into memory.
func ValidateCookies(c *AccountCookies, httpClient *http.Client) (bool, error) {
	req, err := http.NewRequest("GET", geminiAppURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Cookie", BuildCookieHeader(c))
	req.Header.Set("User-Agent", chromeUAValue)

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Scan line-by-line instead of reading entire page into memory
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "SNlM0e") {
			return true, nil
		}
	}
	return false, scanner.Err()
}

const chromeUAValue = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// ChromeUA returns the Chrome User-Agent string.
func ChromeUA() string { return chromeUAValue }

// RotatePSIDTS attempts to rotate the __Secure-1PSIDTS cookie.
func RotatePSIDTS(c *AccountCookies, httpClient *http.Client) (string, error) {
	req, err := http.NewRequest("POST", rotateCookieURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Cookie", BuildCookieHeader(c))
	req.Header.Set("User-Agent", chromeUAValue)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Extract new PSIDTS from Set-Cookie headers
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "__Secure-1PSIDTS" {
			return cookie.Value, nil
		}
	}
	return "", fmt.Errorf("no __Secure-1PSIDTS in rotation response")
}
