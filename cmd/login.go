package cmd

import (
	"fmt"
	"os"

	"github.com/harris/gemini-web-cli/internal/auth"
	"github.com/spf13/cobra"
)

var loginAccount string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Google via browser",
	RunE:  runLogin,
}

func init() {
	loginCmd.Flags().StringVar(&loginAccount, "account", "default", "Account name")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	cookies, err := auth.BrowserLogin(loginAccount)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		return err
	}

	store, err := auth.NewStore()
	if err != nil {
		return fmt.Errorf("failed to init store: %w", err)
	}

	if err := store.SaveCookies(cookies); err != nil {
		return fmt.Errorf("failed to save cookies: %w", err)
	}

	if err := store.AddAccount(loginAccount); err != nil {
		return fmt.Errorf("failed to register account: %w", err)
	}

	fmt.Printf("Account %q logged in successfully.\n", loginAccount)
	return nil
}
