package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/harris/gemini-web-cli/internal/api"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	askModel        string
	askGem          string
	askFile         string
	askShowThoughts bool
	askRaw          bool
)

var askCmd = &cobra.Command{
	Use:   "ask [message]",
	Short: "Single question (non-interactive)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runAsk,
}

func init() {
	askCmd.Flags().StringVar(&askModel, "model", "", "Model name")
	askCmd.Flags().StringVar(&askGem, "gem", "", "Gem name to use as system prompt")
	askCmd.Flags().StringVar(&askFile, "file", "", "Upload a file and include it with the question")
	askCmd.Flags().BoolVar(&askShowThoughts, "show-thoughts", false, "Show thinking process")
	askCmd.Flags().BoolVar(&askRaw, "raw", false, "Output raw markdown without terminal rendering")
	rootCmd.AddCommand(askCmd)
}

func runAsk(cmd *cobra.Command, args []string) error {
	client, err := initClient(askModel)
	if err != nil {
		return err
	}
	defer client.Close()

	// Resolve gem name to ID if specified
	var gemID string
	if askGem != "" {
		gemID, err = resolveGemName(client, askGem)
		if err != nil {
			return err
		}
	}

	prompt := strings.Join(args, " ")

	// Upload file if specified
	var files []api.FileRef
	if askFile != "" {
		fmt.Println("Uploading file...")
		ref, uerr := client.UploadFile(askFile)
		if uerr != nil {
			return fmt.Errorf("file upload failed: %w", uerr)
		}
		files = []api.FileRef{ref}
	}

	var result *api.GenerateResult
	err = client.Retry(func() error {
		var rerr error
		result, rerr = client.GenerateFullWithGem(prompt, nil, files, gemID)
		return rerr
	})
	if err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	if askShowThoughts && result.Thoughts != "" {
		fmt.Println("--- Thinking ---")
		fmt.Println(result.Thoughts)
		fmt.Println("--- Response ---")
	}

	// Render output: glamour for terminal, raw with --raw flag or piped output
	if askRaw || !isTTY() {
		fmt.Println(result.Text)
	} else {
		renderTerminal(result.Text)
	}

	for _, img := range result.Images {
		label := "Image"
		if img.Generated {
			label = "Generated Image"
		}
		if img.Title != "" {
			fmt.Printf("[%s: %s] %s\n", label, img.Title, img.URL)
		} else {
			fmt.Printf("[%s] %s\n", label, img.URL)
		}
	}

	return nil
}

func isTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func renderTerminal(text string) {
	width := 100 // Better default for modern terminals
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	} else {
		fmt.Fprintf(os.Stderr, "Warning: Could not detect terminal width: %v, using default %d\n", err, width)
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to create Glamour renderer: %v\n", err)
		fmt.Println(text)
		return
	}
	out, err := r.Render(text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Glamour rendering failed: %v\n", err)
		fmt.Println(text)
		return
	}
	fmt.Print(out)
}
