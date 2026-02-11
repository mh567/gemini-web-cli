package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/harris/gemini-web-cli/internal/auth"
)

// tokenCache holds cached session tokens for fast startup.
type tokenCache struct {
	SNlM0e  string    `json:"snlm0e"`
	Cfb2h   string    `json:"cfb2h"`
	FdrFJe  string    `json:"fdrfje"`
	SavedAt time.Time `json:"saved_at"`
}

const (
	geminiOrigin = "https://gemini.google.com"
	geminiAppURL = "https://gemini.google.com/app"
)

// Client is the Gemini Web API client.
type Client struct {
	httpClient    *http.Client
	cookies       *auth.AccountCookies
	cookieHeader  string // cached cookie header string
	cookieMu      sync.RWMutex
	snlm0e        string // CSRF token
	cfb2h         string // request context
	fdrFJe        string // additional token
	reqID         int64
	model         Model
	MaxRetries    int
	refreshTicker *time.Ticker
	refreshDone   chan struct{}
	lastRotate    time.Time // cooldown: prevent rotation more than once per 60s
	store         *auth.Store
}

// NewClient creates a new API client.
func NewClient(cookies *auth.AccountCookies, model Model) (*Client, error) {
	return NewClientWithProxy(cookies, model, "")
}

// NewClientWithProxy creates a new API client with optional proxy support.
func NewClientWithProxy(cookies *auth.AccountCookies, model Model, proxy string) (*Client, error) {
	httpClient, err := newHTTPClient(proxy)
	if err != nil {
		return nil, fmt.Errorf("create http client: %w", err)
	}
	c := &Client{
		httpClient:   httpClient,
		cookies:      cookies,
		cookieHeader: auth.BuildCookieHeader(cookies),
		reqID:        rand.Int63n(9000) + 1000,
		model:        model,
		MaxRetries:   3,
	}
	return c, nil
}

// newHTTPClient creates an HTTP client with optional proxy support.
func newHTTPClient(proxy string) (*http.Client, error) {
	transport := &http.Transport{}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{
		Timeout:   120 * time.Second,
		Transport: transport,
	}, nil
}

// chromeHeaders sets standard Chrome browser headers on a request.
func (c *Client) chromeHeaders(req *http.Request) {
	c.cookieMu.RLock()
	header := c.cookieHeader
	c.cookieMu.RUnlock()

	req.Header.Set("User-Agent", auth.ChromeUA())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cookie", header)
	req.Header.Set("Origin", geminiOrigin)
	req.Header.Set("Referer", geminiAppURL)
}

var (
	reSnlm0e = regexp.MustCompile(`"SNlM0e":"([^"]+)"`)
	reCfb2h  = regexp.MustCompile(`"cfb2h":"([^"]+)"`)
	reFdrFJe = regexp.MustCompile(`"FdrFJe":"([^"]+)"`)
)

// tokenCachePath returns the path to the token cache file.
func tokenCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".config", "gemini-web-cli")
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		dir = filepath.Join(xdg, "gemini-web-cli")
	}
	return filepath.Join(dir, "token_cache.json")
}

func (c *Client) loadTokenCache() bool {
	path := tokenCachePath()
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var cache tokenCache
	if json.Unmarshal(data, &cache) != nil {
		return false
	}
	// Cache valid for 30 minutes
	if time.Since(cache.SavedAt) > 30*time.Minute {
		return false
	}
	if cache.SNlM0e == "" {
		return false
	}
	c.snlm0e = cache.SNlM0e
	c.cfb2h = cache.Cfb2h
	c.fdrFJe = cache.FdrFJe
	return true
}

func (c *Client) saveTokenCache() {
	path := tokenCachePath()
	if path == "" {
		return
	}
	cache := tokenCache{
		SNlM0e:  c.snlm0e,
		Cfb2h:   c.cfb2h,
		FdrFJe:  c.fdrFJe,
		SavedAt: time.Now(),
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	_ = os.WriteFile(path, data, 0600)
}

// Init loads cached tokens if fresh, otherwise fetches from Gemini page.
// Uses a short per-attempt timeout (10s) with up to 2 retries for network fetch.
func (c *Client) Init() error {
	// Try cached tokens first (instant)
	if c.loadTokenCache() {
		return nil
	}

	const (
		initTimeout = 10 * time.Second
		maxRetries  = 2
	)
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		lastErr = c.initOnce(initTimeout)
		if lastErr == nil {
			c.saveTokenCache()
			return nil
		}
		if i < maxRetries-1 {
			time.Sleep(2 * time.Second)
		}
	}
	return lastErr
}

func (c *Client) initOnce(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", geminiAppURL, nil)
	if err != nil {
		return err
	}
	c.chromeHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch Gemini page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	html := string(body)

	if m := reSnlm0e.FindStringSubmatch(html); len(m) > 1 {
		c.snlm0e = m[1]
	} else {
		return fmt.Errorf("SNlM0e token not found - cookies may be invalid")
	}

	if m := reCfb2h.FindStringSubmatch(html); len(m) > 1 {
		c.cfb2h = m[1]
	}
	if m := reFdrFJe.FindStringSubmatch(html); len(m) > 1 {
		c.fdrFJe = m[1]
	}

	return nil
}

// nextReqID returns the next request ID and increments it atomically.
func (c *Client) nextReqID() int64 {
	return atomic.AddInt64(&c.reqID, 100000) - 100000
}

// SetModel changes the active model.
func (c *Client) SetModel(m Model) {
	c.model = m
}

// HTTPClient returns the underlying HTTP client (for cookie validation).
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// IsInitialized returns whether the client has valid tokens.
func (c *Client) IsInitialized() bool {
	return c.snlm0e != ""
}

// SNlM0e returns the CSRF token.
func (c *Client) SNlM0e() string {
	return c.snlm0e
}

// SetTimeout updates the HTTP client timeout.
func (c *Client) SetTimeout(seconds int) {
	if seconds > 0 {
		c.httpClient.Timeout = time.Duration(seconds) * time.Second
	}
}

// SetStore sets the credential store for cookie persistence during refresh.
func (c *Client) SetStore(store *auth.Store) {
	c.store = store
}

// StartCookieRefresh starts a background goroutine that rotates PSIDTS every 9 minutes.
func (c *Client) StartCookieRefresh() {
	c.refreshTicker = time.NewTicker(9 * time.Minute)
	c.refreshDone = make(chan struct{})
	go func() {
		for {
			select {
			case <-c.refreshDone:
				return
			case <-c.refreshTicker.C:
				c.rotateCookie()
			}
		}
	}()
}

func (c *Client) rotateCookie() {
	c.cookieMu.Lock()
	defer c.cookieMu.Unlock()

	// Cooldown: skip if last rotation was less than 60 seconds ago
	if time.Since(c.lastRotate) < 60*time.Second {
		return
	}

	newPSIDTS, err := auth.RotatePSIDTS(c.cookies, c.httpClient)
	if err != nil {
		return
	}
	c.lastRotate = time.Now()
	c.cookies.PSIDTS = newPSIDTS
	c.cookieHeader = auth.BuildCookieHeader(c.cookies)

	// Persist updated cookies if store is available
	if c.store != nil {
		_ = c.store.SaveCookies(c.cookies)
	}
}

// Close stops the cookie refresh ticker and cleans up resources.
func (c *Client) Close() {
	if c.refreshTicker != nil {
		c.refreshTicker.Stop()
		close(c.refreshDone)
	}
}

// Retry executes fn with exponential backoff for retryable GeminiErrors.
func (c *Client) Retry(fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		var gemErr *GeminiError
		if !errors.As(lastErr, &gemErr) || !gemErr.IsRetryable() {
			return lastErr
		}
		if attempt < c.MaxRetries {
			time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
		}
	}
	return lastErr
}
