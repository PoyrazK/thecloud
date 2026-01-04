package main

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Manage CloudGateway (API Routes)",
}

var createRouteCmd = &cobra.Command{
	Use:   "create-route [name] [prefix] [target]",
	Short: "Create a new gateway route",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		strip, _ := cmd.Flags().GetBool("strip")
		limit, _ := cmd.Flags().GetInt("rate-limit")

		client := getClient()
		route, err := client.CreateGatewayRoute(args[0], args[1], args[2], strip, limit)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Route created: %s (Prefix: %s -> %s)\n", route.Name, route.PathPrefix, route.TargetURL)
	},
}

var listRoutesCmd = &cobra.Command{
	Use:   "list-routes",
	Short: "List all gateway routes",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		routes, err := client.ListGatewayRoutes()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "PREFIX", "TARGET", "STRIP"})
		for _, r := range routes {
			table.Append([]string{r.ID, r.Name, r.PathPrefix, r.TargetURL, fmt.Sprintf("%v", r.StripPrefix)})
		}
		table.Render()
	},
}

var deleteRouteCmd = &cobra.Command{
	Use:   "rm-route [id]",
	Short: "Delete a gateway route",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		err := client.DeleteGatewayRoute(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("[SUCCESS] Route deleted")
	},
}

func init() {
	createRouteCmd.Flags().Bool("strip", true, "Strip prefix from target request")
	createRouteCmd.Flags().Int("rate-limit", 100, "Rate limit (req/sec)")

	gatewayCmd.AddCommand(createRouteCmd)
	gatewayCmd.AddCommand(listRoutesCmd)
	gatewayCmd.AddCommand(deleteRouteCmd)

	rootCmd.AddCommand(gatewayCmd)
}
