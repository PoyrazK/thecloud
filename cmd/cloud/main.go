// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.4.0"

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

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion script",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletion(os.Stdout)
			}
			return fmt.Errorf("unsupported shell: %s", args[0])
		},
	})

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
	rootCmd.AddCommand(sgCmd)
	rootCmd.AddCommand(subnetCmd)
	rootCmd.AddCommand(kubernetesCmd)
	rootCmd.AddCommand(dnsCmd)
	rootCmd.AddCommand(cloudLogsCmd)
	rootCmd.AddCommand(iamCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(billingCmd)
	rootCmd.AddCommand(tenantCmd)
	rootCmd.AddCommand(instanceTypeCmd)
	rootCmd.AddCommand(newSSHKeyCmd(&opts))
	rootCmd.AddCommand(vpcPeeringCmd)
	rootCmd.AddCommand(igwCmd)
	rootCmd.AddCommand(natGatewayCmd)
	rootCmd.AddCommand(routeTableCmd)
	rootCmd.AddCommand(configCmd)

	rootCmd.PersistentFlags().StringVarP(&opts.Output, "output", "o", "table", "Output format (table, json, yaml)")
	rootCmd.PersistentFlags().BoolVarP(&opts.JSON, "json", "j", false, "Output in JSON format (deprecated, use --output=json)")
	rootCmd.PersistentFlags().StringVarP(&opts.APIKey, "api-key", "k", "", "API key for authentication")
	rootCmd.PersistentFlags().StringVar(&opts.APIURL, "api-url", "http://localhost:8080", "URL of the API server")
	rootCmd.PersistentFlags().StringVar(&opts.TenantID, "tenant", "", "Tenant ID to use for requests")
	rootCmd.PersistentFlags().BoolVarP(&opts.Debug, "debug", "d", false, "Enable debug mode")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
