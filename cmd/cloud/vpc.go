package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var vpcCmd = &cobra.Command{
	Use:   "vpc",
	Short: "Manage Virtual Private Clouds (VPCs)",
}

var vpcListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all VPCs",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		vpcs, err := client.ListVPCs()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if outputJSON {
			data, _ := json.MarshalIndent(vpcs, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "CIDR", "VXLAN", "STATUS", "CREATED AT"})

		for _, v := range vpcs {
			table.Append([]string{
				v.ID[:8],
				v.Name,
				v.CIDRBlock,
				fmt.Sprintf("%d", v.VXLANID),
				v.Status,
				v.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		table.Render()
	},
}

var vpcCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new VPC",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		cidr, _ := cmd.Flags().GetString("cidr-block")
		client := getClient()
		vpc, err := client.CreateVPC(name, cidr)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] VPC %s created successfully!\n", vpc.Name)
		fmt.Printf("ID: %s\n", vpc.ID)
		fmt.Printf("CIDR: %s\n", vpc.CIDRBlock)
		fmt.Printf("VXLAN ID: %d\n", vpc.VXLANID)
		fmt.Printf("Network ID: %s\n", vpc.NetworkID)
	},
}

var vpcRmCmd = &cobra.Command{
	Use:   "rm [id/name]",
	Short: "Remove a VPC",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := getClient()
		if err := client.DeleteVPC(id); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] VPC %s removed successfully.\n", id)
	},
}

func init() {
	vpcCreateCmd.Flags().String("cidr-block", "10.0.0.0/16", "CIDR block for the VPC")
	vpcCmd.AddCommand(vpcListCmd)
	vpcCmd.AddCommand(vpcCreateCmd)
	vpcCmd.AddCommand(vpcRmCmd)
}
