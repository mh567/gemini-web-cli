package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/harris/gemini-web-cli/internal/tui"
	"github.com/spf13/cobra"
)

var (
	chatModelFlag string
	chatGemFlag   string
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start interactive chat",
	RunE:  runChat,
}

func init() {
	chatCmd.Flags().StringVar(&chatModelFlag, "model", "", "Model name")
	chatCmd.Flags().StringVar(&chatGemFlag, "gem", "", "Gem name to use as system prompt")
	rootCmd.AddCommand(chatCmd)
}

func runChat(cmd *cobra.Command, args []string) error {
	client, err := createClient(chatModelFlag)
	if err != nil {
		return err
	}
	defer client.Close()

	app := tui.NewApp(client, chatGemFlag)
	p := tea.NewProgram(app)
	_, err = p.Run()
	return err
}
