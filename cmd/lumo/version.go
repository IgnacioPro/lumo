package main

import (
	"fmt"
	"runtime"

	"github.com/ignacio/lumo/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Display the version, build information, and runtime details of the Lumo CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Lumo CLI\n")
		fmt.Printf("  Version:      %s\n", version.Version)
		fmt.Printf("  Go Version:   %s\n", runtime.Version())
		fmt.Printf("  OS/Arch:      %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("  Compiler:     %s\n", runtime.Compiler)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
