package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

// BrowserLogin opens a Chrome browser for the user to log in manually.
// By default it tries to reuse local browser profile state and falls back
// to an isolated profile when unavailable.
func BrowserLogin(account string, isolated bool) (*AccountCookies, error) {
	fmt.Println("Launching Chrome for Google login...")
	fmt.Println("Please log in to your Google account in the browser window.")
	fmt.Printf("Timeout: %v\n\n", loginTimeout)

	u, err := launchBrowser(isolated)
	if err != nil {
		return nil, err
	}

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

func launchBrowser(isolated bool) (string, error) {
	path, _ := launcher.LookPath()

	if !isolated {
		if profileDir, ok := detectProfileDir(); ok {
			u, err := launchWithProfile(path, profileDir)
			if err == nil {
				fmt.Printf("Using local browser profile: %s\n\n", profileDir)
				return u, nil
			}
			fmt.Printf("Failed to reuse local browser profile (%v)\n", err)
			fmt.Println("Falling back to isolated browser profile.")
			fmt.Println()
		} else {
			fmt.Println("Local browser profile not found.")
			fmt.Println("Falling back to isolated browser profile.")
			fmt.Println()
		}
	}

	return launchWithProfile(path, "")
}

func launchWithProfile(binPath, userDataDir string) (string, error) {
	l := launcher.New().
		Headless(false).
		Set("disable-blink-features", "AutomationControlled")

	if binPath != "" {
		l = l.Bin(binPath)
	}
	if userDataDir != "" {
		l = l.UserDataDir(userDataDir)
	}

	u, err := l.Launch()
	if err != nil {
		return "", fmt.Errorf("failed to launch browser: %w", err)
	}
	return u, nil
}

func detectProfileDir() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}

	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			filepath.Join(home, "Library", "Application Support", "Google", "Chrome"),
			filepath.Join(home, "Library", "Application Support", "Chromium"),
		}
	default:
		candidates = []string{
			filepath.Join(home, ".config", "google-chrome"),
			filepath.Join(home, ".config", "chromium"),
		}
	}

	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if err == nil && info.IsDir() {
			return dir, true
		}
	}
	return "", false
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
