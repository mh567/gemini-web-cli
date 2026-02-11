package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

const (
	geminiURL    = "https://gemini.google.com/app"
	loginTimeout = 5 * time.Minute
)

// RequiredCookieNames lists the cookies we need to extract.
var RequiredCookieNames = []string{
	"__Secure-1PSID",
	"__Secure-1PSIDTS",
	"__Secure-1PSIDCC",
}

// BrowserLogin opens a real Chrome browser for the user to log in manually.
// It waits for login to complete, then extracts the required cookies.
func BrowserLogin(account string) (*AccountCookies, error) {
	fmt.Println("Launching Chrome for Google login...")
	fmt.Println("Please log in to your Google account in the browser window.")
	fmt.Printf("Timeout: %v\n\n", loginTimeout)

	// Find or download Chrome
	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).
		Headless(false).
		Set("disable-blink-features", "AutomationControlled").
		MustLaunch()

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage(geminiURL)
	defer page.MustClose()

	// Wait for login: poll until we're on gemini.google.com/app with cookies
	cookies, err := waitForLogin(page)
	if err != nil {
		return nil, err
	}

	cookies.Account = account
	fmt.Println("\nLogin successful! Cookies extracted.")
	return cookies, nil
}

func waitForLogin(page *rod.Page) (*AccountCookies, error) {
	deadline := time.Now().Add(loginTimeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("login timed out after %v", loginTimeout)
			}
			info := page.MustInfo()
			if !strings.Contains(info.URL, "gemini.google.com") {
				continue
			}
			cookies, err := extractCookies(page)
			if err == nil {
				return cookies, nil
			}
		}
	}
}

func extractCookies(page *rod.Page) (*AccountCookies, error) {
	browserCookies, err := page.Cookies([]string{geminiURL})
	if err != nil {
		return nil, err
	}

	cookieMap := make(map[string]string)
	for _, c := range browserCookies {
		cookieMap[c.Name] = c.Value
	}

	// Check all required cookies are present
	for _, name := range RequiredCookieNames {
		if cookieMap[name] == "" {
			return nil, fmt.Errorf("missing cookie: %s", name)
		}
	}

	return &AccountCookies{
		PSID:   cookieMap["__Secure-1PSID"],
		PSIDTS: cookieMap["__Secure-1PSIDTS"],
		PSIDCC: cookieMap["__Secure-1PSIDCC"],
		NID:    cookieMap["NID"],
	}, nil
}
