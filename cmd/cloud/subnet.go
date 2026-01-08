package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var subnetCmd = &cobra.Command{
	Use:   "subnet",
	Short: "Manage Subnets within VPCs",
}

var subnetListCmd = &cobra.Command{
	Use:   "list [vpc-id]",
	Short: "List subnets in a VPC",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		vpcID := args[0]
		subnets, err := client.ListSubnets(vpcID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if outputJSON {
			data, _ := json.MarshalIndent(subnets, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "CIDR", "AZ", "GATEWAY", "STATUS", "CREATED AT"})

		for _, s := range subnets {
			table.Append([]string{
				s.ID[:8],
				s.Name,
				s.CIDRBlock,
				s.AZ,
				s.GatewayIP,
				s.Status,
				s.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		table.Render()
	},
}

var subnetCreateCmd = &cobra.Command{
	Use:   "create [vpc-id] [name] [cidr]",
	Short: "Create a new subnet in a VPC",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		vpcID := args[0]
		name := args[1]
		cidr := args[2]
		az, _ := cmd.Flags().GetString("az")

		subnet, err := client.CreateSubnet(vpcID, name, cidr, az)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("Subnet created successfully: %s (%s)\n", subnet.Name, subnet.ID)
	},
}

var subnetDeleteCmd = &cobra.Command{
	Use:   "rm [subnet-id]",
	Short: "Remove a subnet",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		subnetID := args[0]

		err := client.DeleteSubnet(subnetID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("Subnet %s deleted successfully\n", subnetID)
	},
}

// init configures the subnet create command's flags and registers subnet subcommands.
// It adds the "az" flag with default "us-east-1a" to subnetCreateCmd and attaches
// subnetListCmd, subnetCreateCmd, and subnetDeleteCmd to subnetCmd.
func init() {
	subnetCreateCmd.Flags().String("az", "us-east-1a", "Availability zone")

	subnetCmd.AddCommand(subnetListCmd)
	subnetCmd.AddCommand(subnetCreateCmd)
	subnetCmd.AddCommand(subnetDeleteCmd)
}