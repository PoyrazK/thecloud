package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

func newSSHKeyCmd(o *CLIOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh-key",
		Short: "Manage SSH public keys",
	}

	cmd.AddCommand(newSSHKeyRegisterCmd(o))
	cmd.AddCommand(newSSHKeyListCmd(o))
	return cmd
}

func newSSHKeyRegisterCmd(o *CLIOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "register [name] [public_key_file]",
		Short: "Register a new SSH public key",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			keyFile := args[1]

			keyData, err := os.ReadFile(filepath.Clean(keyFile))
			if err != nil {
				fmt.Printf("Error reading key file: %v\n", err)
				return
			}

			client := createClient(*o)
			key, err := client.RegisterSSHKey(name, string(keyData))
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			fmt.Println("[SUCCESS] SSH Key registered.")
			fmt.Printf("ID: %s\n", key.ID)
		},
	}
}

func newSSHKeyListCmd(o *CLIOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all registered SSH keys",
		Run: func(cmd *cobra.Command, args []string) {
			client := createClient(*o)

			limit, _ := cmd.Flags().GetInt("limit")
			offset, _ := cmd.Flags().GetInt("offset")

			var keys []sdk.SSHKey
			var meta *sdk.ListResponse[sdk.SSHKey]

			if limit > 0 || offset > 0 {
				var err error
				keys, meta, err = client.ListSSHKeysWithPagination(limit, offset)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					return
				}
			} else {
				var err error
				keys, err = client.ListSSHKeys()
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					return
				}
			}

			for _, k := range keys {
				fmt.Printf("%-36s %s\n", k.ID, k.Name)
			}

			if meta != nil {
				fmt.Printf("\nShowing %d of %d total", len(keys), meta.TotalCount)
				if meta.HasMore {
					fmt.Print(" (more available)")
				}
				fmt.Println()
			}
		},
	}
	cmd.Flags().Int("limit", 0, "Maximum number of results (0 = use server default)")
	cmd.Flags().Int("offset", 0, "Number of results to skip")
	return cmd
}
