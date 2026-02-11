package cmd

import (
	"fmt"
	"os"

	"github.com/harris/gemini-web-cli/pkg/version"
	"github.com/spf13/cobra"
)

var proxyFlag string

var rootCmd = &cobra.Command{
	Use:   "gemini-web-cli",
	Short: "Terminal client for Gemini using Google One subscription",
	Long:  "Gemini Web CLI is a terminal tool for interacting with Google Gemini using your Google One web subscription.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&proxyFlag, "proxy", "", "Proxy URL (e.g. socks5://127.0.0.1:1080)")
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Full())
	},
}
