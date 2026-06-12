package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("BOSS-cli version %s\n", version)
			fmt.Printf("  commit:    %s\n", commit)
			fmt.Printf("  buildDate: %s\n", buildDate)
			fmt.Printf("  go:        %s\n", runtime.Version())
			fmt.Printf("  os/arch:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	})
}
