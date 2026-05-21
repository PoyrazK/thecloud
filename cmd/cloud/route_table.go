// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

const rtErrorFormat = "Error: %v\n"

var routeTableCmd = &cobra.Command{
	Use:   "route-table",
	Short: "Manage VPC Route Tables",
}

var routeTableListCmd = &cobra.Command{
	Use:   "list [vpc-id]",
	Short: "List route tables for a VPC",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		vpcID := args[0]
		rts, err := client.ListRouteTables(vpcID)
		if err != nil {
			fmt.Printf(rtErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(rts)
			return
		}

		if len(rts) == 0 {
			fmt.Println("No route tables found.")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "VPC ID", "MAIN", "CREATED AT"})

		for _, rt := range rts {
			mainStr := "No"
			if rt.IsMain {
				mainStr = "Yes"
			}
			_ = table.Append([]string{
				truncateID(rt.ID),
				rt.Name,
				truncateID(rt.VPCID),
				mainStr,
				rt.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		_ = table.Render()
	},
}

var routeTableCreateCmd = &cobra.Command{
	Use:   "create [vpc-id] [name]",
	Short: "Create a new route table",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		vpcID := args[0]
		name := args[1]
		isMain, _ := cmd.Flags().GetBool("main")

		rt, err := client.CreateRouteTable(vpcID, name, isMain)
		if err != nil {
			fmt.Printf(rtErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(rt)
		} else {
			fmt.Printf("[SUCCESS] Route table %s created successfully!\n", rt.Name)
			fmt.Printf("ID: %s\n", rt.ID)
			fmt.Printf("VPC ID: %s\n", rt.VPCID)
			fmt.Printf("Main: %t\n", rt.IsMain)
		}
	},
}

var routeTableRmCmd = &cobra.Command{
	Use:     "rm [rt-id]",
	Aliases: []string{"delete"},
	Short:   "Remove a route table",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		rtID := args[0]
		if err := client.DeleteRouteTable(rtID); err != nil {
			fmt.Printf(rtErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Route table %s removed successfully.\n", rtID)
	},
}

var routeTableAddRouteCmd = &cobra.Command{
	Use:   "add-route [rt-id] [destination-cidr] [target-type]",
	Short: "Add a route to a route table",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		rtID := args[0]
		destCIDR := args[1]
		targetType := sdk.RouteTargetType(args[2])
		targetID, _ := cmd.Flags().GetString("target-id")

		route, err := client.AddRoute(rtID, destCIDR, targetType, targetID)
		if err != nil {
			fmt.Printf(rtErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(route)
		} else {
			fmt.Printf("[SUCCESS] Route %s added successfully!\n", route.ID)
			fmt.Printf("Destination: %s\n", route.DestinationCIDR)
			fmt.Printf("Target Type: %s\n", route.TargetType)
		}
	},
}

var routeTableAssociateCmd = &cobra.Command{
	Use:   "associate [rt-id] [subnet-id]",
	Short: "Associate a subnet with a route table",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		rtID := args[0]
		subnetID := args[1]

		if err := client.AssociateSubnet(rtID, subnetID); err != nil {
			fmt.Printf(rtErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Subnet %s associated with route table %s\n", subnetID, rtID)
	},
}

var routeTableDisassociateCmd = &cobra.Command{
	Use:   "disassociate [rt-id] [subnet-id]",
	Short: "Disassociate a subnet from a route table",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		rtID := args[0]
		subnetID := args[1]

		if err := client.DisassociateSubnet(rtID, subnetID); err != nil {
			fmt.Printf(rtErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Subnet %s disassociated from route table %s\n", subnetID, rtID)
	},
}

func init() {
	routeTableCreateCmd.Flags().Bool("main", false, "Set as the main route table")

	routeTableCmd.AddCommand(routeTableListCmd)
	routeTableCmd.AddCommand(routeTableCreateCmd)
	routeTableCmd.AddCommand(routeTableRmCmd)
	routeTableCmd.AddCommand(routeTableAddRouteCmd)
	routeTableCmd.AddCommand(routeTableAssociateCmd)
	routeTableCmd.AddCommand(routeTableDisassociateCmd)

	rootCmd.AddCommand(routeTableCmd)
}
