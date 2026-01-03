package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage encrypted secrets and configurations",
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secrets (values redacted)",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		secrets, err := client.ListSecrets()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if outputJSON {
			data, _ := json.MarshalIndent(secrets, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "DESCRIPTION", "CREATED AT"})

		for _, s := range secrets {
			id := s.ID
			if len(id) > 8 {
				id = id[:8]
			}

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
	Use:   "create",
	Short: "Store a new encrypted secret",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		value, _ := cmd.Flags().GetString("value")
		desc, _ := cmd.Flags().GetString("description")

		client := getClient()
		secret, err := client.CreateSecret(name, value, desc)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Secret %s created.\n", name)
		if outputJSON {
			data, _ := json.MarshalIndent(secret, "", "  ")
			fmt.Println(string(data))
		}
	},
}

var secretsGetCmd = &cobra.Command{
	Use:   "get [id/name]",
	Short: "Decrypt and show a secret value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := getClient()
		secret, err := client.GetSecret(id)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if outputJSON {
			data, _ := json.MarshalIndent(secret, "", "  ")
			fmt.Println(string(data))
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
	Use:   "rm [id/name]",
	Short: "Remove a secret",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := getClient()
		if err := client.DeleteSecret(id); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("[SUCCESS] Secret removed.")
	},
}

func init() {
	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsCreateCmd)
	secretsCmd.AddCommand(secretsGetCmd)
	secretsCmd.AddCommand(secretsRmCmd)

	secretsCreateCmd.Flags().StringP("name", "n", "", "Unique name of the secret (required)")
	secretsCreateCmd.Flags().StringP("value", "v", "", "Value to encrypt (required)")
	secretsCreateCmd.Flags().StringP("description", "d", "", "Optional description")
	secretsCreateCmd.MarkFlagRequired("name")
	secretsCreateCmd.MarkFlagRequired("value")
}
