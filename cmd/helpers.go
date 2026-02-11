package cmd

import (
	"fmt"
	"strings"

	"github.com/harris/gemini-web-cli/internal/api"
	"github.com/harris/gemini-web-cli/internal/auth"
	"github.com/harris/gemini-web-cli/internal/config"
)

// createClient loads cookies and creates an API client without initializing the session.
func createClient(modelName string) (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	store, err := auth.NewStore()
	if err != nil {
		return nil, fmt.Errorf("store error: %w", err)
	}

	account := cfg.DefaultAccount
	list, err := store.ListAccounts()
	if err == nil && list.Default != "" {
		account = list.Default
	}

	cookies, err := store.LoadCookies(account)
	if err != nil {
		return nil, fmt.Errorf(
			"no credentials for account %q. Run: gemini-web-cli login",
			account,
		)
	}

	// Determine model
	model := api.DefaultModel()
	if modelName == "" {
		modelName = cfg.DefaultModel
	}
	if modelName != "" {
		if m, ok := api.GetModel(modelName); ok {
			model = m
		}
	}

	// Determine proxy: CLI flag > config > env
	proxy := proxyFlag
	if proxy == "" {
		proxy = cfg.Proxy
	}

	client, err := api.NewClientWithProxy(cookies, model, proxy)
	if err != nil {
		return nil, err
	}
	client.SetStore(store)
	client.SetTimeout(cfg.RequestTimeout)
	return client, nil
}

// initClient loads cookies, creates an API client, and initializes the session.
func initClient(modelName string) (*api.Client, error) {
	client, err := createClient(modelName)
	if err != nil {
		return nil, err
	}

	fmt.Println("Initializing session...")
	if err := client.Init(); err != nil {
		return nil, fmt.Errorf(
			"session init failed (cookies may be expired): %w\n"+
				"Run: gemini-web-cli login", err,
		)
	}

	client.StartCookieRefresh()
	return client, nil
}

// resolveGemName finds a Gem by name and returns its ID.
func resolveGemName(client *api.Client, name string) (string, error) {
	gems, err := client.ListGems()
	if err != nil {
		return "", fmt.Errorf("failed to list gems: %w", err)
	}
	for _, g := range gems {
		if strings.EqualFold(g.Name, name) {
			return g.ID, nil
		}
	}
	return "", fmt.Errorf("gem %q not found", name)
}
