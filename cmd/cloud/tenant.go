// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var tenantCmd = &cobra.Command{
	Use:   "tenant",
	Short: "Manage tenant organizations",
}

var tenantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tenants you belong to",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		tenants, err := client.ListTenants()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if opts.JSON {
			printJSON(tenants)
			return
		}

		if len(tenants) == 0 {
			fmt.Println("No tenants found.")
			return
		}

		table := tablewriter.NewWriter(cmd.OutOrStdout())
		table.Header([]string{"ID", "NAME", "SLUG", "STATUS", "CREATED AT"})

		for _, t := range tenants {
			_ = table.Append([]string{
				t.ID,
				t.Name,
				t.Slug,
				t.Status,
				t.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
		_ = table.Render()
	},
}

var tenantCreateCmd = &cobra.Command{
	Use:   "create [name] [slug]",
	Short: "Create a new tenant organization",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name, slug := args[0], args[1]
		client := createClient(opts)
		tenant, err := client.CreateTenant(name, slug)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Tenant %s (%s) created successfully!\n", tenant.Name, tenant.ID)
	},
}

var tenantSwitchCmd = &cobra.Command{
	Use:   "switch [id]",
	Short: "Switch your default tenant",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.SwitchTenant(id); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Switched to tenant %s.\n", id)
	},
}

func init() {
	tenantCmd.AddCommand(tenantListCmd)
	tenantCmd.AddCommand(tenantCreateCmd)
	tenantCmd.AddCommand(tenantSwitchCmd)
}
