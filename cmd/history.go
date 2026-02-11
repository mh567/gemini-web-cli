package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "List conversation history",
	RunE:  runHistory,
}

func init() {
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
	client, err := initClient("")
	if err != nil {
		return err
	}

	convs, err := client.ListConversations()
	if err != nil {
		return fmt.Errorf("failed to list conversations: %w", err)
	}

	if len(convs) == 0 {
		fmt.Println("No conversations found.")
		return nil
	}

	for i, c := range convs {
		fmt.Printf("%d. %s\n   ID: %s\n", i+1, c.Title, c.ID)
	}
	return nil
}
