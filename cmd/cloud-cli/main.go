package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Mini AWS Cloud CLI",
	Long:  `A local-first cloud simulator CLI for learning and development.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of cloud CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cloud v%s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(computeCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(storageCmd)
	rootCmd.AddCommand(vpcCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
