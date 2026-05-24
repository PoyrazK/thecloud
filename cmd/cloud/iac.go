// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const cliErrorFormat = "Error: %v\n"

var iacCmd = &cobra.Command{
	Use:   "iac",
	Short: "Manage Infrastructure as Code (IaC) Stacks",
}

var iacListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all stacks",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		stacks, err := client.ListStacks()
		if err != nil {
			fmt.Printf(cliErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(stacks)
			return
		}

		if len(stacks) == 0 {
			fmt.Println("No stacks found.")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "STATUS", "CREATED AT"})

		for _, s := range stacks {
			_ = table.Append([]string{
				truncateID(s.ID.String()),
				s.Name,
				string(s.Status),
				s.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		_ = table.Render()
	},
}

var iacCreateCmd = &cobra.Command{
	Use:   "create [name] [template_path]",
	Short: "Create a new stack from a template file",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		templatePath := args[1]

		templateData, err := os.ReadFile(filepath.Clean(templatePath))
		if err != nil {
			fmt.Printf("Error reading template file: %v\n", err)
			return
		}

		client := createClient(opts)
		stack, err := client.CreateStack(name, string(templateData), nil)
		if err != nil {
			fmt.Printf(cliErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(stack)
		} else {
			fmt.Printf("[SUCCESS] Stack %s creation initiated!\n", stack.Name)
			fmt.Printf("ID: %s\n", stack.ID)
			fmt.Printf("Status: %s\n", stack.Status)
		}
	},
}

var iacGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get stack details and resources",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		stack, err := client.GetStack(args[0])
		if err != nil {
			fmt.Printf(cliErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(stack)
			return
		}

		fmt.Printf("Stack: %s (%s)\n", stack.Name, stack.ID)
		fmt.Printf("Status: %s\n", stack.Status)
		if stack.StatusReason != "" {
			fmt.Printf("Reason: %s\n", stack.StatusReason)
		}
		fmt.Printf("Created: %s\n\n", stack.CreatedAt.Format("2006-01-02 15:04:05"))

		if len(stack.Resources) > 0 {
			fmt.Println("Resources:")
			table := tablewriter.NewWriter(os.Stdout)
			table.Header([]string{"LOGICAL ID", "PHYSICAL ID", "TYPE", "STATUS"})
			for _, r := range stack.Resources {
				_ = table.Append([]string{
					r.LogicalID,
					r.PhysicalID,
					r.ResourceType,
					r.Status,
				})
			}
			_ = table.Render()
		}
	},
}

var iacRmCmd = &cobra.Command{
	Use:     "rm [id]",
	Aliases: []string{"delete"},
	Short:   "Remove a stack and its resources",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.DeleteStack(id); err != nil {
			fmt.Printf(cliErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Stack %s deletion initiated.\n", id)
	},
}

var iacValidateCmd = &cobra.Command{
	Use:   "validate [template_path]",
	Short: "Validate an IaC template",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		templatePath := args[0]
		templateData, err := os.ReadFile(filepath.Clean(templatePath))
		if err != nil {
			fmt.Printf("Error reading template file: %v\n", err)
			return
		}

		client := createClient(opts)
		resp, err := client.ValidateTemplate(string(templateData))
		if err != nil {
			fmt.Printf(cliErrorFormat, err)
			return
		}

		if resp.Valid {
			fmt.Println("[SUCCESS] Template is valid.")
		} else {
			fmt.Println("[FAILED] Template validation errors:")
			for _, e := range resp.Errors {
				fmt.Printf("- %s\n", e)
			}
		}
	},
}

func init() {
	iacCmd.AddCommand(iacListCmd)
	iacCmd.AddCommand(iacCreateCmd)
	iacCmd.AddCommand(iacGetCmd)
	iacCmd.AddCommand(iacRmCmd)
	iacCmd.AddCommand(iacValidateCmd)
}
