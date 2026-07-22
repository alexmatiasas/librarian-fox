package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Mostrar versión",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("lfx %s (commit %s)\n", version, commit)
		},
	}
}
