package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "cloud",
	Short: "The Cloud Cloud CLI",
	Long:  `A local-first cloud simulator CLI for learning and development.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of cloud CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cloud v%s\n", version)
	},
}

// init registers the CLI's top-level subcommands on rootCmd.
// It adds versionCmd and the service commands: instance, auth, storage, vpc,
// lb, db, secrets, function, cache, autoscaling, container, cron, events,
// gateway, notify, queue, snapshot, volume, iac, roles, and subnet.
func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(instanceCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(storageCmd)
	rootCmd.AddCommand(vpcCmd)
	rootCmd.AddCommand(lbCmd)
	rootCmd.AddCommand(dbCmd)
	rootCmd.AddCommand(secretsCmd)
	rootCmd.AddCommand(functionCmd)
	rootCmd.AddCommand(cacheCmd)
	rootCmd.AddCommand(autoscalingCmd)
	rootCmd.AddCommand(containerCmd)
	rootCmd.AddCommand(cronCmd)
	rootCmd.AddCommand(eventsCmd)
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(notifyCmd)
	rootCmd.AddCommand(queueCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(volumeCmd)
	rootCmd.AddCommand(iacCmd)
	rootCmd.AddCommand(rolesCmd)
	rootCmd.AddCommand(subnetCmd)
}

// main is the program entry point; it executes the root CLI command and exits with status 1 if execution fails.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}