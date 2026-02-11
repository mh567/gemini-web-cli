package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var gemsCmd = &cobra.Command{
	Use:   "gems",
	Short: "Manage Gems (system prompts)",
}

var gemsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Gems",
	RunE:  runGemsList,
}

func init() {
	gemsCreateCmd.Flags().StringVar(&gemCreateName, "name", "", "Gem name (required)")
	gemsCreateCmd.Flags().StringVar(&gemCreatePrompt, "prompt", "", "System prompt (required)")
	gemsCreateCmd.Flags().StringVar(&gemCreateDesc, "desc", "", "Description")
	_ = gemsCreateCmd.MarkFlagRequired("name")
	_ = gemsCreateCmd.MarkFlagRequired("prompt")

	gemsCmd.AddCommand(gemsListCmd)
	gemsCmd.AddCommand(gemsCreateCmd)
	gemsCmd.AddCommand(gemsDeleteCmd)
	rootCmd.AddCommand(gemsCmd)
}

var (
	gemCreateName   string
	gemCreatePrompt string
	gemCreateDesc   string
)

var gemsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Gem",
	RunE:  runGemsCreate,
}

var gemsDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a Gem by ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runGemsDelete,
}

func runGemsList(cmd *cobra.Command, args []string) error {
	client, err := initClient("")
	if err != nil {
		return err
	}
	defer client.Close()

	gems, err := client.ListGems()
	if err != nil {
		return err
	}

	if len(gems) == 0 {
		fmt.Println("No gems found.")
		return nil
	}

	for _, g := range gems {
		prefix := " "
		if g.Predefined {
			prefix = "*"
		}
		fmt.Printf("%s %-20s %s\n", prefix, g.Name, g.ID)
	}
	return nil
}

func runGemsCreate(cmd *cobra.Command, args []string) error {
	client, err := initClient("")
	if err != nil {
		return err
	}
	defer client.Close()

	id, err := client.CreateGem(gemCreateName, gemCreatePrompt, gemCreateDesc)
	if err != nil {
		return err
	}

	fmt.Printf("Created gem %q (ID: %s)\n", gemCreateName, id)
	return nil
}

func runGemsDelete(cmd *cobra.Command, args []string) error {
	client, err := initClient("")
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.DeleteGem(args[0]); err != nil {
		return err
	}

	fmt.Println("Gem deleted.")
	return nil
}