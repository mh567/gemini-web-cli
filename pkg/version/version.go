package version

import "fmt"

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func Full() string {
	return fmt.Sprintf("gemini-web-cli %s (commit: %s, built: %s)", Version, GitCommit, BuildDate)
}
