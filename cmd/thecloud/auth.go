package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

var listRolesCmd = &cobra.Command{
	Use:   "roles",
	Short: "List available roles",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		roles, err := client.ListRoles()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		for _, role := range roles {
			fmt.Println(role)
		}
	},
}

var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Inspect or update user roles",
}

var getRoleCmd = &cobra.Command{
	Use:   "get [user-id|me]",
	Short: "Get a user's role (defaults to current user)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		if len(args) == 0 || args[0] == "me" {
			role, err := client.GetMyRole()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			fmt.Printf("%s %s\n", role.UserID, role.Role)
			return
		}

		role, err := client.GetUserRole(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("%s %s\n", role.UserID, role.Role)
	},
}

var setRoleCmd = &cobra.Command{
	Use:   "set <user-id> <role>",
	Short: "Update a user's role",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		role, err := client.UpdateUserRole(args[0], args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("%s %s\n", role.UserID, role.Role)
	},
}

var createDemoCmd = &cobra.Command{
	Use:   "create-demo [name]",
	Short: "Generate a demo API key and save it",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		client := sdk.NewClient(apiURL, "") // Key not needed for creation usually
		key, err := client.CreateKey(name)

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[INFO] Generated Key: %s\n", key)
		saveConfig(key)
		fmt.Println("[SUCCESS] Key saved to configuration. You can now use 'cloud' commands without flags.")
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

// Config persistence
type Config struct {
	APIKey string `json:"api_key"`
}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".thecloud", "config.json")
}

func saveConfig(key string) {
	path := getConfigPath()
	os.MkdirAll(filepath.Dir(path), 0755)

	cfg := Config{APIKey: key}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)
}

func loadConfig() string {
	path := getConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cfg Config
	json.Unmarshal(data, &cfg)
	return cfg.APIKey
}

func init() {
	authCmd.AddCommand(createDemoCmd)
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(listRolesCmd)
	authCmd.AddCommand(roleCmd)
	roleCmd.AddCommand(getRoleCmd)
	roleCmd.AddCommand(setRoleCmd)
}
