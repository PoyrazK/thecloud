package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type cliConfig struct {
	APICredential string `json:"auth"`
	APIURL        string `json:"api_url"`
	Output        string `json:"output"`
	Tenant        string `json:"tenant"`
	Debug         bool   `json:"debug"`
	// LegacyAPIKey for backward compatibility with existing config files
	LegacyAPIKey string `json:"api_key,omitempty"` //#nosec G117
}

// UnmarshalJSON handles both old ("api_key") and new ("auth") JSON tags
func (c *cliConfig) UnmarshalJSON(data []byte) error {
	type alias cliConfig
	tmp := struct {
		*alias
		LegacyAPIKey string `json:"api_key,omitempty"`
	}{alias: (*alias)(c)}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	if c.APICredential == "" && tmp.LegacyAPIKey != "" {
		c.APICredential = tmp.LegacyAPIKey
	}
	return nil
}

func getConfigFilePath() string {
	if path := os.Getenv("CLOUD_CONFIG_PATH"); path != "" {
		return path
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cloud", "config.json")
}

func loadConfigFile() string {
	configPath := getConfigFilePath()
	data, err := os.ReadFile(configPath) //#nosec G304
	if err != nil {
		return ""
	}

	var cfg cliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}

	return cfg.APICredential
}

func loadFullConfig() *cliConfig {
	configPath := getConfigFilePath()
	data, err := os.ReadFile(configPath) //#nosec G304
	if err != nil {
		return &cliConfig{}
	}

	var cfg cliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &cliConfig{}
	}

	return &cfg
}

func saveConfigFile(cfg cliConfig) error {
	configPath := getConfigFilePath()
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ") //#nosec G117
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadFullConfig()
		printOutput(cfg)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadFullConfig()
		key := args[0]
		value := args[1]

		switch key {
		case "api-key":
			cfg.APICredential = value
		case "api-url":
			cfg.APIURL = value
		case "output":
			cfg.Output = value
		case "tenant":
			cfg.Tenant = value
		default:
			fmt.Printf("Error: unknown config key: %s\n", key)
			fmt.Println("Valid keys: api-key, api-url, output, tenant")
			os.Exit(1)
		}

		if err := saveConfigFile(*cfg); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("[SUCCESS] configuration updated")
	},
}

var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Unset a configuration value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadFullConfig()
		key := args[0]

		switch key {
		case "api-key":
			cfg.APICredential = ""
		case "api-url":
			cfg.APIURL = ""
		case "output":
			cfg.Output = ""
		case "tenant":
			cfg.Tenant = ""
		default:
			fmt.Printf("Error: unknown config key: %s\n", key)
			fmt.Println("Valid keys: api-key, api-url, output, tenant")
			os.Exit(1)
		}

		if err := saveConfigFile(*cfg); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("[SUCCESS] %s unset\n", key)
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configUnsetCmd)
}
