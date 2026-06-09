// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const natErrorFormat = "Error: %v\n"

var natGatewayCmd = &cobra.Command{
	Use:   "nat-gateway",
	Short: "Manage NAT Gateways",
}

var natGatewayCreateCmd = &cobra.Command{
	Use:   "create [subnet-id] [eip-id]",
	Short: "Create a NAT gateway in a subnet with an elastic IP",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		subnetID := args[0]
		eipID := args[1]

		nat, err := client.CreateNATGateway(subnetID, eipID)
		if err != nil {
			fmt.Printf(natErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(nat)
		} else {
			fmt.Printf("[SUCCESS] NAT Gateway %s created successfully!\n", nat.ID)
			fmt.Printf("ID: %s\n", nat.ID)
			fmt.Printf("Subnet ID: %s\n", nat.SubnetID)
			fmt.Printf("Elastic IP ID: %s\n", nat.ElasticIPID)
			fmt.Printf("Private IP: %s\n", nat.PrivateIP)
			fmt.Printf("Status: %s\n", nat.Status)
		}
	},
}

var natGatewayListCmd = &cobra.Command{
	Use:   "list [vpc-id]",
	Short: "List NAT gateways for a VPC",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		vpcID := args[0]

		nats, err := client.ListNATGateways(vpcID)
		if err != nil {
			fmt.Printf(natErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(nats)
			return
		}

		if len(nats) == 0 {
			fmt.Println("No NAT gateways found.")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "SUBNET ID", "EIP ID", "PRIVATE IP", "STATUS", "CREATED AT"})

		for _, nat := range nats {
			_ = table.Append([]string{
				truncateID(nat.ID),
				truncateID(nat.SubnetID),
				truncateID(nat.ElasticIPID),
				nat.PrivateIP,
				string(nat.Status),
				nat.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		_ = table.Render()
	},
}

var natGatewayRmCmd = &cobra.Command{
	Use:     "rm [nat-id]",
	Aliases: []string{"delete"},
	Short:   "Delete a NAT gateway",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		natID := args[0]

		if err := client.DeleteNATGateway(natID); err != nil {
			fmt.Printf(natErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] NAT Gateway %s deleted successfully.\n", natID)
	},
}

func init() {
	natGatewayCmd.AddCommand(natGatewayCreateCmd)
	natGatewayCmd.AddCommand(natGatewayListCmd)
	natGatewayCmd.AddCommand(natGatewayRmCmd)

	rootCmd.AddCommand(natGatewayCmd)
}
