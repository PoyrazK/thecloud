// Package main provides the cloud CLI entrypoint.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var vpcPeeringCmd = &cobra.Command{
	Use:   "vpc-peering",
	Short: "Manage VPC peering connections",
}

var vpcPeeringCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Request a new VPC peering connection",
	Run: func(cmd *cobra.Command, args []string) {
		reqVpc, _ := cmd.Flags().GetString("requester-vpc")
		accVpc, _ := cmd.Flags().GetString("accepter-vpc")
		accTenant, _ := cmd.Flags().GetString("accepter-tenant")

		if reqVpc == "" || accVpc == "" {
			fmt.Println("Error: --requester-vpc and --accepter-vpc are both required")
			return
		}
		if accTenant == "" {
			fmt.Println("Error: --accepter-tenant is required for cross-tenant peering")
			return
		}

		client := createClient(opts)
		peering, err := client.CreateVPCPeering(reqVpc, accVpc, accTenant)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		fmt.Printf("[SUCCESS] VPC Peering request %s created successfully!\n", peering.ID)
		fmt.Printf("Status: %s\n", peering.Status)
		fmt.Printf("ARN: %s\n", peering.ARN)
	},
}

var vpcPeeringListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all VPC peering connections",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		peerings, err := client.ListVPCPeerings()
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			data, err := json.MarshalIndent(peerings, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				return
			}
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "REQUESTER VPC", "ACCEPTER VPC", "STATUS", "CREATED AT"})

		for _, p := range peerings {
			table.Append([]string{
				truncateID(p.ID),
				truncateID(p.RequesterVPCID),
				truncateID(p.AccepterVPCID),
				p.Status,
				p.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
		table.Render()
	},
}

var vpcPeeringGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get details of a VPC peering connection",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		p, err := client.GetVPCPeering(args[0])
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			data, err := json.MarshalIndent(p, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				return
			}
			fmt.Println(string(data))
			return
		}

		fmt.Printf("VPC Peering: %s\n", p.ID)
		fmt.Printf("Requester VPC: %s\n", p.RequesterVPCID)
		fmt.Printf("Accepter VPC: %s\n", p.AccepterVPCID)
		fmt.Printf("Requester Tenant: %s\n", p.RequesterTenantID)
		fmt.Printf("Accepter Tenant: %s\n", p.AccepterTenantID)
		fmt.Printf("Status: %s\n", p.Status)
		fmt.Printf("ARN: %s\n", p.ARN)
		fmt.Printf("Created At: %s\n", p.CreatedAt.String())
		fmt.Printf("Updated At: %s\n", p.UpdatedAt.String())
	},
}

var vpcPeeringAcceptCmd = &cobra.Command{
	Use:   "accept [id]",
	Short: "Accept a pending VPC peering connection",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		peering, err := client.AcceptVPCPeering(args[0])
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		fmt.Printf("[SUCCESS] VPC Peering %s accepted and active!\n", peering.ID)
	},
}

var vpcPeeringRejectCmd = &cobra.Command{
	Use:   "reject [id]",
	Short: "Reject a pending VPC peering connection",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		if err := client.RejectVPCPeering(args[0]); err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		fmt.Printf("[SUCCESS] VPC Peering %s rejected.\n", args[0])
	},
}

var vpcPeeringRmCmd = &cobra.Command{
	Use:     "rm [id]",
	Aliases: []string{"delete"},
	Short:   "Delete/Disconnect a VPC peering connection",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		if err := client.DeleteVPCPeering(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] VPC Peering %s deleted.\n", args[0])
	},
}

func init() {
	vpcPeeringCreateCmd.Flags().String("requester-vpc", "", "Requester VPC ID")
	vpcPeeringCreateCmd.Flags().String("accepter-vpc", "", "Accepter VPC ID")
	vpcPeeringCreateCmd.Flags().String("accepter-tenant", "", "Accepter VPC Tenant ID (required for cross-tenant peering)")

	vpcPeeringCmd.AddCommand(
		vpcPeeringCreateCmd,
		vpcPeeringListCmd,
		vpcPeeringGetCmd,
		vpcPeeringAcceptCmd,
		vpcPeeringRejectCmd,
		vpcPeeringRmCmd,
	)
}
