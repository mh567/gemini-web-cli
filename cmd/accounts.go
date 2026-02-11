package cmd

import (
	"fmt"

	"github.com/harris/gemini-web-cli/internal/auth"
	"github.com/spf13/cobra"
)

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "Manage accounts",
}

func init() {
	accountsCmd.AddCommand(accountsListCmd)
	accountsCmd.AddCommand(accountsSwitchCmd)
	accountsCmd.AddCommand(accountsRemoveCmd)
	rootCmd.AddCommand(accountsCmd)
}

var accountsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := auth.NewStore()
		if err != nil {
			return err
		}
		list, err := store.ListAccounts()
		if err != nil {
			return err
		}
		if len(list.Accounts) == 0 {
			fmt.Println("No accounts. Run: gemini-web-cli login")
			return nil
		}
		for _, a := range list.Accounts {
			marker := "  "
			if a == list.Default {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, a)
		}
		return nil
	},
}

var accountsSwitchCmd = &cobra.Command{
	Use:   "switch [name]",
	Short: "Switch default account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := auth.NewStore()
		if err != nil {
			return err
		}
		if err := store.SetDefault(args[0]); err != nil {
			return err
		}
		fmt.Printf("Switched to account %q\n", args[0])
		return nil
	},
}

var accountsRemoveCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove an account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := auth.NewStore()
		if err != nil {
			return err
		}
		if err := store.RemoveAccount(args[0]); err != nil {
			return err
		}
		fmt.Printf("Removed account %q\n", args[0])
		return nil
	},
}
