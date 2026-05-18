// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const vpcErrorFormat = "Error: %v\n"

var vpcCmd = &cobra.Command{
	Use:   "vpc",
	Short: "Manage Virtual Private Clouds (VPCs)",
}

var vpcListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all VPCs",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		vpcs, err := client.ListVPCs()
		if err != nil {
			fmt.Printf(vpcErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(vpcs)
			return
		}

		if len(vpcs) == 0 {
			fmt.Println("No VPCs found.")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "CIDR", "VXLAN", "STATUS", "CREATED AT"})

		for _, v := range vpcs {
			table.Append([]string{
				truncateID(v.ID),
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
		client := createClient(opts)
		vpc, err := client.CreateVPC(name, cidr)
		if err != nil {
			fmt.Printf(vpcErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(vpc)
		} else {
			fmt.Printf("[SUCCESS] VPC %s created successfully!\n", vpc.Name)
			fmt.Printf("ID: %s\n", vpc.ID)
			fmt.Printf("CIDR: %s\n", vpc.CIDRBlock)
			fmt.Printf("VXLAN ID: %d\n", vpc.VXLANID)
			fmt.Printf("Network ID: %s\n", vpc.NetworkID)
		}
	},
}

var vpcRmCmd = &cobra.Command{
	Use:     "rm [id/name]",
	Aliases: []string{"delete"},
	Short:   "Remove a VPC",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.DeleteVPC(id); err != nil {
			fmt.Printf(vpcErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] VPC %s removed successfully.\n", id)
	},
}

var vpcShowCmd = &cobra.Command{
	Use:   "show [id/name]",
	Short: "Show VPC details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		vpc, err := client.GetVPC(id)
		if err != nil {
			fmt.Printf(vpcErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(vpc)
			return
		}

		fmt.Print("\nVPC Details\n")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf(fmtDetailRow, "ID:", vpc.ID)
		fmt.Printf(fmtDetailRow, "Name:", vpc.Name)
		fmt.Printf(fmtDetailRow, "CIDR:", vpc.CIDRBlock)
		fmt.Printf(fmtDetailRow, "VXLAN ID:", fmt.Sprintf("%d", vpc.VXLANID))
		fmt.Printf(fmtDetailRow, "Network ID:", vpc.NetworkID)
		fmt.Printf(fmtDetailRow, "Status:", vpc.Status)
		fmt.Printf(fmtDetailRow, "Created At:", vpc.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Println()
	},
}

func init() {
	vpcCreateCmd.Flags().String("cidr-block", "10.0.0.0/16", "CIDR block for the VPC")
	vpcCmd.AddCommand(vpcListCmd)
	vpcCmd.AddCommand(vpcCreateCmd)
	vpcCmd.AddCommand(vpcRmCmd)
	vpcCmd.AddCommand(vpcShowCmd)
}
