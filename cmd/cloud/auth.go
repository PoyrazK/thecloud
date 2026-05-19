// Package main provides the cloud CLI entrypoint.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

var createDemoCmd = &cobra.Command{
	Use:   "create-demo [name]",
	Short: "Generate a demo API key and save it",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		client := sdk.NewClient(opts.APIURL, "") // Key not needed for creation usually
		key, err := client.CreateKey(name)

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[INFO] Generated Key: %s\n", key.Key)
		saveConfig(key.Key)
		fmt.Println("[SUCCESS] Key saved to configuration. You can now use 'cloud' commands without flags.")
	},
}

var listApiKeysCmd = &cobra.Command{
	Use:   "list-keys",
	Short: "List your API keys",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		keys, err := client.ListKeys()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if opts.JSON {
			printJSON(keys)
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "CREATED AT", "LAST USED"})
		for _, k := range keys {
			if err := table.Append([]string{
				k.ID.String(),
				k.Name,
				k.CreatedAt.Format("2006-01-02"),
				k.LastUsed.Format("2006-01-02"),
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

var revokeKeyCmd = &cobra.Command{
	Use:   "revoke-key [id]",
	Short: "Revoke an API key",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		if err := client.RevokeKey(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("[SUCCESS] Key revoked.")
	},
}

var rotateKeyCmd = &cobra.Command{
	Use:   "rotate-key [id]",
	Short: "Rotate an API key",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		key, err := client.RotateKey(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("[SUCCESS] Key rotated. New Key: %s\n", key.Key)
		fmt.Println("[INFO] Remember to update your saved config if this was your active key.")
	},
}

var loginCmd = &cobra.Command{
	Use:   "login [key]",
	Short: "Save an existing API key to configuration",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		saveConfig(key)
		fmt.Println("[SUCCESS] Key saved to configuration.")
	},
}

var registerCmd = &cobra.Command{
	Use:   "register [email] [password] [name]",
	Short: "Register a new user account",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		email, password, name := args[0], args[1], args[2]
		client := sdk.NewClient(opts.APIURL, "")
		user, err := client.Register(email, password, name)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("[SUCCESS] User %s (%s) registered successfully!\n", user.Name, user.Email)
		fmt.Println("[INFO] Please log in with 'cloud auth login-user <email> <password>' to get an API key.")
	},
}

var loginUserCmd = &cobra.Command{
	Use:   "login-user [email] [password]",
	Short: "Login with email and password to get an API key",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		email, password := args[0], args[1]
		client := sdk.NewClient(opts.APIURL, "")
		res, err := client.Login(email, password)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		saveConfig(res.APIKey)
		fmt.Printf("[SUCCESS] Logged in as %s. Key saved to configuration.\n", res.User.Email)
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current session information",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		user, err := client.WhoAmI()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if opts.JSON {
			printJSON(user)
			return
		}

		fmt.Println("Current User:")
		fmt.Printf("  ID:       %s\n", user.ID)
		fmt.Printf("  Name:     %s\n", user.Name)
		fmt.Printf("  Email:    %s\n", user.Email)
		fmt.Printf("  Role:     %s\n", user.Role)
		if user.DefaultTenantID != nil {
			fmt.Printf("  TenantID: %s\n", *user.DefaultTenantID)
		}
	},
}

// Config persistence
type Config struct {
	APIKey string `json:"api_key"`
}

func getConfigPath() string {
	return getConfigFilePath()
}

func saveConfig(key string) {
	path := getConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		fmt.Printf("Warning: failed to create config directory: %v\n", err)
	}

	cfg := Config{APIKey: key}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Printf("Error: failed to marshal config: %v\n", err)
		return
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		fmt.Printf("Error: failed to write config file: %v\n", err)
	}
}

func loadConfig() string {
	path := getConfigPath()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return ""
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.APIKey
}

func init() {
	authCmd.AddCommand(createDemoCmd)
	authCmd.AddCommand(listApiKeysCmd)
	authCmd.AddCommand(revokeKeyCmd)
	authCmd.AddCommand(rotateKeyCmd)
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(registerCmd)
	authCmd.AddCommand(loginUserCmd)
	authCmd.AddCommand(whoamiCmd)
}
