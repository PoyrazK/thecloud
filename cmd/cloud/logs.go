package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

var cloudLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Manage platform logs",
}

var logsSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search and filter logs",
	Run: func(cmd *cobra.Command, args []string) {
		resID, _ := cmd.Flags().GetString("resource-id")
		resType, _ := cmd.Flags().GetString("resource-type")
		level, _ := cmd.Flags().GetString("level")
		search, _ := cmd.Flags().GetString("query")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")

		query := sdk.LogQuery{
			ResourceID:   resID,
			ResourceType: resType,
			Level:        level,
			Search:       search,
			Limit:        limit,
			Offset:       offset,
		}

		client := createClient(opts)
		res, err := client.SearchLogs(cmd.Context(), query)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if opts.JSON {
			data, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling response: %v\n", err)
				return
			}
			fmt.Println(string(data))
			return
		}

		displayLogs(res.Entries)
	},
}

var logsShowCmd = &cobra.Command{
	Use:   "show [resource-id]",
	Short: "Show logs for a specific resource",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		limit, _ := cmd.Flags().GetInt("limit")

		client := createClient(opts)
		res, err := client.GetLogsByResource(cmd.Context(), id, limit)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if opts.JSON {
			data, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling response: %v\n", err)
				return
			}
			fmt.Println(string(data))
			return
		}

		displayLogs(res.Entries)
	},
}

func displayLogs(entries []sdk.LogEntry) {
	if len(entries) == 0 {
		fmt.Println("No logs found.")
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"TIMESTAMP", "LEVEL", "RESOURCE", "MESSAGE"})

	for _, e := range entries {
		resource := e.ResourceType
		if e.ResourceID != "" {
			shortID := e.ResourceID
			if len(shortID) > 8 {
				shortID = truncateID(shortID)
			}
			resource = fmt.Sprintf("%s/%s", e.ResourceType, shortID)
		}

		_ = table.Append([]string{
			e.Timestamp.Format(time.RFC3339),
			e.Level,
			resource,
			e.Message,
		})
	}
	_ = table.Render()
}

func init() {
	cloudLogsCmd.AddCommand(logsSearchCmd)
	cloudLogsCmd.AddCommand(logsShowCmd)

	logsSearchCmd.Flags().String("resource-id", "", "Filter by resource ID")
	logsSearchCmd.Flags().String("resource-type", "", "Filter by resource type (instance, function)")
	logsSearchCmd.Flags().String("level", "", "Filter by log level (INFO, WARN, ERROR)")
	logsSearchCmd.Flags().String("query", "", "Search keyword in message")
	logsSearchCmd.Flags().Int("limit", 100, "Limit number of logs")
	logsSearchCmd.Flags().Int("offset", 0, "Offset for pagination")

	logsShowCmd.Flags().Int("limit", 100, "Limit number of logs")
}
