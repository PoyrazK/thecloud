package main

import (
	"encoding/json"
	"fmt"
	"os"

	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/poyraz/cloud/pkg/sdk"
	"github.com/spf13/cobra"
)

var apiURL = "http://localhost:8080"
var outputJSON bool
var apiKey string

var computeCmd = &cobra.Command{
	Use:   "compute",
	Short: "Manage compute instances",
}

func getClient() *sdk.Client {
	key := apiKey // 1. Flag
	if key == "" {
		key = os.Getenv("MINIAWS_API_KEY") // 2. Env Var
	}
	if key == "" {
		key = loadConfig() // 3. Config File
	}

	if key == "" {
		fmt.Println("[WARN] No API Key found. Run 'cloud auth create-demo <name>' to get one.")
		os.Exit(1)
	}

	return sdk.NewClient(apiURL, key)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all instances",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		instances, err := client.ListInstances()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if outputJSON {
			data, _ := json.MarshalIndent(instances, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "IMAGE", "STATUS", "ACCESS"})

		for _, inst := range instances {
			id := inst.ID
			if len(id) > 8 {
				id = id[:8]
			}

			access := "-"
			if inst.Ports != "" && inst.Status == "RUNNING" {
				pList := strings.Split(inst.Ports, ",")
				var mappings []string
				for _, mapping := range pList {
					parts := strings.Split(mapping, ":")
					if len(parts) == 2 {
						mappings = append(mappings, fmt.Sprintf("localhost:%s->%s", parts[0], parts[1]))
					}
				}
				access = strings.Join(mappings, ", ")
			}

			table.Append([]string{
				id,
				inst.Name,
				inst.Image,
				inst.Status,
				access,
			})
		}
		table.Render()
	},
}

var launchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch a new instance",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		image, _ := cmd.Flags().GetString("image")
		ports, _ := cmd.Flags().GetString("port")
		vpc, _ := cmd.Flags().GetString("vpc")

		client := getClient()
		inst, err := client.LaunchInstance(name, image, ports, vpc)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Instance launched successfully!\n")
		data, _ := json.MarshalIndent(inst, "", "  ")
		fmt.Println(string(data))
	},
}
var stopCmd = &cobra.Command{
	Use:   "stop [id]",
	Short: "Stop an instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := getClient()
		if err := client.StopInstance(id); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Println("[INFO] Instance stop initiated.")
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs [id]",
	Short: "View instance logs",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := getClient()
		logs, err := client.GetInstanceLogs(id)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Print(logs)
	},
}

var showCmd = &cobra.Command{
	Use:   "show [id/name]",
	Short: "Show detailed instance information",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := getClient()
		inst, err := client.GetInstance(id)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("\nInstance Details\n")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("%-15s %v\n", "ID:", inst.ID)
		fmt.Printf("%-15s %v\n", "Name:", inst.Name)
		fmt.Printf("%-15s %v\n", "Status:", inst.Status)
		fmt.Printf("%-15s %v\n", "Image:", inst.Image)
		fmt.Printf("%-15s %v\n", "Ports:", inst.Ports)
		fmt.Printf("%-15s %v\n", "Created At:", inst.CreatedAt)
		fmt.Printf("%-15s %v\n", "Version:", inst.Version)
		fmt.Printf("%-15s %v\n", "Container ID:", inst.ContainerID)
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println("")
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm [id/name]",
	Short: "Remove an instance and its resources",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := getClient()
		if err := client.TerminateInstance(id); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

	},
}

var statsCmd = &cobra.Command{
	Use:   "stats [id/name]",
	Short: "Show instance statistics (CPU/Mem)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := getClient()
		stats, err := client.GetInstanceStats(id)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if outputJSON {
			data, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Printf("\nStatistics for %s\n", id)
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("CPU:    %.2f%%\n", stats.CPUPercentage)
		fmt.Printf("Memory: %.2f%% (%.2f MB / %.2f MB)\n",
			stats.MemoryPercentage,
			stats.MemoryUsageBytes/1024/1024,
			stats.MemoryLimitBytes/1024/1024)
	},
}

func init() {
	computeCmd.AddCommand(listCmd)
	computeCmd.AddCommand(launchCmd)
	computeCmd.AddCommand(stopCmd)
	computeCmd.AddCommand(logsCmd)
	computeCmd.AddCommand(showCmd)
	computeCmd.AddCommand(rmCmd)
	computeCmd.AddCommand(statsCmd)

	launchCmd.Flags().StringP("name", "n", "", "Name of the instance (required)")
	launchCmd.Flags().StringP("image", "i", "alpine", "Image to use")
	launchCmd.Flags().StringP("port", "p", "", "Port mapping (host:container)")
	launchCmd.Flags().StringP("vpc", "v", "", "VPC ID or Name to attach to")
	launchCmd.MarkFlagRequired("name")

	rootCmd.PersistentFlags().BoolVarP(&outputJSON, "json", "j", false, "Output in JSON format")
	rootCmd.PersistentFlags().StringVarP(&apiKey, "api-key", "k", "", "API key for authentication")
}
