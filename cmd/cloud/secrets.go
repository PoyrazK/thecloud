// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const secretsErrorFormat = "Error: %v\n"

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage encrypted secrets and configurations",
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secrets (values redacted)",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		secrets, err := client.ListSecrets()
		if err != nil {
			fmt.Printf(secretsErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(secrets)
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "DESCRIPTION", "CREATED AT"})

		for _, s := range secrets {
			id := truncateID(s.ID)

			table.Append([]string{
				id,
				s.Name,
				s.Description,
				s.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		table.Render()
	},
}

var secretsCreateCmd = &cobra.Command{
	Use:   "create [name] [value]",
	Short: "Store a new encrypted secret",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		value := args[1]
		desc, _ := cmd.Flags().GetString("description")

		client := createClient(opts)
		secret, err := client.CreateSecret(name, value, desc)
		if err != nil {
			fmt.Printf(secretsErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(secret)
		} else {
			fmt.Printf("[SUCCESS] Secret %s created.\n", name)
		}
	},
}

var secretsGetCmd = &cobra.Command{
	Use:   "get [id/name]",
	Short: "Decrypt and show a secret value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		secret, err := client.GetSecret(id)
		if err != nil {
			fmt.Printf(secretsErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(secret)
		} else {
			fmt.Printf("Name:        %s\n", secret.Name)
			fmt.Printf("Value:       %s\n", secret.EncryptedValue) // This is the decrypted value from service
			fmt.Printf("Description: %s\n", secret.Description)
			fmt.Printf("Created At:  %v\n", secret.CreatedAt)
			if secret.LastAccessedAt != nil {
				fmt.Printf("Last Accessed: %v\n", secret.LastAccessedAt)
			}
		}
	},
}

var secretsRmCmd = &cobra.Command{
	Use:     "rm [id/name]",
	Aliases: []string{"delete"},
	Short:   "Remove a secret",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.DeleteSecret(id); err != nil {
			fmt.Printf(secretsErrorFormat, err)
			return
		}
		fmt.Printf("[SUCCESS] Secret %s removed.\n", id)
	},
}

func init() {
	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsCreateCmd)
	secretsCmd.AddCommand(secretsGetCmd)
	secretsCmd.AddCommand(secretsRmCmd)

	secretsCreateCmd.Flags().StringP("name", "n", "", "Unique name of the secret")
	secretsCreateCmd.Flags().StringP("value", "v", "", "Value to encrypt")
	secretsCreateCmd.Flags().StringP("description", "d", "", "Optional description")
}
