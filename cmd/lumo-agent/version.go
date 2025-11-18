package main

import (
	"fmt"
	"runtime"

	"github.com/ignacio/lumo/internal/agent"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Display the version, build information, and runtime details of the Lumo agent.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Lumo Agent\n")
		fmt.Printf("  Version:      %s\n", agent.Version)
		fmt.Printf("  Go Version:   %s\n", runtime.Version())
		fmt.Printf("  OS/Arch:      %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("  Compiler:     %s\n", runtime.Compiler)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
