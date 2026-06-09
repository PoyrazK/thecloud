// Package main provides the cloud CLI entrypoint.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/spf13/cobra"
)

var iamCmd = &cobra.Command{
	Use:   "iam",
	Short: "Manage Identity and Access Management (IAM) policies",
}

var iamPolicyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all IAM policies",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		policies, err := client.ListPolicies(cmd.Context())
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if opts.JSON {
			printJSON(policies)
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "DESCRIPTION"})

		for _, p := range policies {
			if err := table.Append([]string{
				truncateID(p.ID.String()),
				p.Name,
				p.Description,
			}); err != nil {
				fmt.Printf("Error appending to table: %v\n", err)
				return
			}
		}
		if err := table.Render(); err != nil {
			fmt.Printf("Error rendering table: %v\n", err)
		}
	},
}

var iamPolicyCreateCmd = &cobra.Command{
	Use:   "create [name] [json_file]",
	Short: "Create a new IAM policy from a JSON file",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		filePath := args[1]

		data, err := os.ReadFile(filePath) //nolint:gosec // intentional file loading from user-provided path
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			return
		}

		var statements []domain.Statement
		if err := json.Unmarshal(data, &statements); err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			return
		}

		desc, _ := cmd.Flags().GetString("description")

		policy := &domain.Policy{
			Name:        name,
			Description: desc,
			Statements:  statements,
		}

		client := createClient(opts)
		newPolicy, err := client.CreatePolicy(cmd.Context(), policy)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Policy %s created successfully (ID: %s)\n", newPolicy.Name, newPolicy.ID)
	},
}

var iamPolicyAttachCmd = &cobra.Command{
	Use:   "attach [user_id] [policy_id]",
	Short: "Attach a policy to a user",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		userID, err := uuid.Parse(args[0])
		if err != nil {
			fmt.Printf("Error: invalid user ID: %v\n", err)
			return
		}
		policyID, err := uuid.Parse(args[1])
		if err != nil {
			fmt.Printf("Error: invalid policy ID: %v\n", err)
			return
		}

		client := createClient(opts)
		if err := client.AttachPolicyToUser(cmd.Context(), userID, policyID); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Println("[SUCCESS] Policy attached to user.")
	},
}

var iamPolicyDetachCmd = &cobra.Command{
	Use:   "detach [user_id] [policy_id]",
	Short: "Detach a policy from a user",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		userID, err := uuid.Parse(args[0])
		if err != nil {
			fmt.Printf("Error: invalid user ID: %v\n", err)
			return
		}
		policyID, err := uuid.Parse(args[1])
		if err != nil {
			fmt.Printf("Error: invalid policy ID: %v\n", err)
			return
		}

		client := createClient(opts)
		if err := client.DetachPolicyFromUser(cmd.Context(), userID, policyID); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Println("[SUCCESS] Policy detached from user.")
	},
}

func init() {
	iamPolicyCreateCmd.Flags().String("description", "", "Policy description")

	iamCmd.AddCommand(iamPolicyListCmd)
	iamCmd.AddCommand(iamPolicyCreateCmd)
	iamCmd.AddCommand(iamPolicyAttachCmd)
	iamCmd.AddCommand(iamPolicyDetachCmd)
}
