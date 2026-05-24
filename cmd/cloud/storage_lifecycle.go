// Package main provides the cloud CLI commands.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var lifecycleCmd = &cobra.Command{
	Use:   "lifecycle",
	Short: "Manage bucket lifecycle rules",
}

var lifecycleSetCmd = &cobra.Command{
	Use:   "set [bucket]",
	Short: "Create or update a lifecycle rule (expiration)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bucket := args[0]
		prefix, _ := cmd.Flags().GetString("prefix")
		days, _ := cmd.Flags().GetInt("days")
		enabled, _ := cmd.Flags().GetBool("enabled")

		if days < 1 {
			fmt.Println("Error: --days must be at least 1")
			return
		}

		client := createClient(opts)
		rule, err := client.CreateLifecycleRule(bucket, prefix, days, enabled)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(rule, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Printf("[SUCCESS] Lifecycle rule created for bucket %s\n", bucket)
		fmt.Printf("ID: %s\nPrefix: %s\nExpires: %d days\nEnabled: %v\n", rule.ID, rule.Prefix, rule.ExpirationDays, rule.Enabled)
	},
}

var lifecycleListCmd = &cobra.Command{
	Use:   "list [bucket]",
	Short: "List lifecycle rules for a bucket",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bucket := args[0]
		client := createClient(opts)
		rules, err := client.ListLifecycleRules(bucket)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(rules, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "PREFIX", "DAYS", "ENABLED", "CREATED AT"})

		for _, r := range rules {
			_ = table.Append([]string{
				r.ID,
				r.Prefix,
				fmt.Sprintf("%d", r.ExpirationDays),
				fmt.Sprintf("%v", r.Enabled),
				r.CreatedAt.Format(time.RFC3339),
			})
		}
		_ = table.Render()
	},
}

var lifecycleDeleteCmd = &cobra.Command{
	Use:     "rm [bucket] [rule-id]",
	Aliases: []string{"delete"},
	Short:   "Delete a lifecycle rule",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		bucket := args[0]
		ruleID := args[1]
		client := createClient(opts)
		if err := client.DeleteLifecycleRule(bucket, ruleID); err != nil {
			fmt.Printf(errFmt, err)
			return
		}
		fmt.Printf("[SUCCESS] Deleted lifecycle rule %s from bucket %s\n", ruleID, bucket)
	},
}

func init() {
	storageCmd.AddCommand(lifecycleCmd)
	lifecycleCmd.AddCommand(lifecycleSetCmd)
	lifecycleCmd.AddCommand(lifecycleListCmd)
	lifecycleCmd.AddCommand(lifecycleDeleteCmd)

	lifecycleSetCmd.Flags().String("prefix", "", "Object key prefix")
	lifecycleSetCmd.Flags().Int("days", 30, "Expiration days")
	lifecycleSetCmd.Flags().Bool("enabled", true, "Enable rule immediately")
}
