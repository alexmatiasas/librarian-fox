package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "lfx",
		Short: "Clasificador, restaurador y organizador de documentos",
		Long:  "Unifica una colección desordenada de archivos de lectura en una librería organizada.",
	}

	rootCmd.AddCommand(
		newVersionCmd(),
		newConfigCmd(),
		newScanCmd(),
		// newClassifyCmd(),
		// newRestoreCmd(),
		// newApplyCmd(),
		// newServeCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
