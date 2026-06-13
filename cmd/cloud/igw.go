// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const igwErrorFormat = "Error: %v\n"

var igwCmd = &cobra.Command{
	Use:   "igw",
	Short: "Manage Internet Gateways",
}

var igwCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an internet gateway",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		igw, err := client.CreateIGW()
		if err != nil {
			fmt.Printf(igwErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(igw)
		} else {
			fmt.Printf("[SUCCESS] Internet Gateway %s created successfully!\n", igw.ID)
			fmt.Printf("ID: %s\n", igw.ID)
			fmt.Printf("Status: %s\n", igw.Status)
		}
	},
}

var igwAttachCmd = &cobra.Command{
	Use:   "attach [igw-id] [vpc-id]",
	Short: "Attach an internet gateway to a VPC",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		igwID := args[0]
		vpcID := args[1]

		if err := client.AttachIGW(igwID, vpcID); err != nil {
			fmt.Printf(igwErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Internet Gateway %s attached to VPC %s\n", igwID, vpcID)
	},
}

var igwDetachCmd = &cobra.Command{
	Use:   "detach [igw-id]",
	Short: "Detach an internet gateway from its VPC",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		igwID := args[0]

		if err := client.DetachIGW(igwID); err != nil {
			fmt.Printf(igwErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Internet Gateway %s detached successfully.\n", igwID)
	},
}

var igwListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all internet gateways",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		igws, err := client.ListIGWs()
		if err != nil {
			fmt.Printf(igwErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(igws)
			return
		}

		if len(igws) == 0 {
			fmt.Println("No internet gateways found.")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "VPC ID", "STATUS", "CREATED AT"})

		for _, igw := range igws {
			vpcIDStr := "None"
			if igw.VPCID != nil {
				vpcIDStr = truncateID(*igw.VPCID)
			}
			_ = table.Append([]string{
				truncateID(igw.ID),
				vpcIDStr,
				string(igw.Status),
				igw.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		_ = table.Render()
	},
}

var igwRmCmd = &cobra.Command{
	Use:     "rm [igw-id]",
	Aliases: []string{"delete"},
	Short:   "Delete an internet gateway (must be detached)",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		igwID := args[0]

		if err := client.DeleteIGW(igwID); err != nil {
			fmt.Printf(igwErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Internet Gateway %s deleted successfully.\n", igwID)
	},
}

func init() {
	igwCmd.AddCommand(igwCreateCmd)
	igwCmd.AddCommand(igwAttachCmd)
	igwCmd.AddCommand(igwDetachCmd)
	igwCmd.AddCommand(igwListCmd)
	igwCmd.AddCommand(igwRmCmd)

	rootCmd.AddCommand(igwCmd)
}
