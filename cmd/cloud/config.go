package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type cliConfig struct {
	APIKey string `json:"api_key"`
	APIURL string `json:"api_url"`
	Output string `json:"output"`
	Tenant string `json:"tenant"`
	Debug  bool   `json:"debug"`
}

func getConfigFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cloud", "config.json")
}

var configFile = getConfigFilePath()

func loadConfigFile() string {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return ""
	}

	var cfg cliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}

	return cfg.APIKey
}

func loadFullConfig() *cliConfig {
	data, err := os.ReadFile(configFile)
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
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0600); err != nil {
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
			cfg.APIKey = value
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
			cfg.APIKey = ""
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
