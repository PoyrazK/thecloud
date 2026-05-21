// Package main provides the cloud CLI entrypoint.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var fnSchedCmd = &cobra.Command{
	Use:   "fn-schedule",
	Short: "Manage Function Schedules",
}

var createFnSchedCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new function schedule",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		functionRef, _ := cmd.Flags().GetString("function")
		schedule, _ := cmd.Flags().GetString("schedule")
		payloadStr, _ := cmd.Flags().GetString("payload")

		// Validate JSON payload
		if payloadStr != "{}" && !json.Valid([]byte(payloadStr)) {
			return fmt.Errorf("invalid JSON payload: %s", payloadStr)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		client := createClient(opts)

		// Resolve function name to ID
		fnID := functionRef
		if _, err := uuid.Parse(functionRef); err != nil {
			// Not a UUID, try to resolve by name
			functions, err := client.ListFunctionsContext(ctx)
			if err != nil {
				return fmt.Errorf("failed to list functions: %w", err)
			}
			found := false
			for _, f := range functions {
				if f.Name == functionRef {
					fnID = f.ID
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("function %q not found", functionRef)
			}
		}

		payload := []byte(payloadStr)
		sched, err := client.CreateFunctionScheduleContext(ctx, fnID, name, schedule, payload)
		if err != nil {
			return err
		}

		fmt.Printf("Schedule %s created (ID: %s)\n", sched.Name, sched.ID)
		if sched.NextRunAt != "" {
			fmt.Printf("Next run: %s\n", sched.NextRunAt)
		}
		return nil
	},
}

var listFnSchedCmd = &cobra.Command{
	Use:   "list",
	Short: "List all function schedules",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createClient(opts)
		schedules, err := client.ListFunctionSchedules()
		if err != nil {
			return err
		}

		if len(schedules) == 0 {
			fmt.Println("No function schedules found.")
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "FUNCTION", "SCHEDULE", "STATUS", "NEXT RUN"})
		for _, s := range schedules {
			_ = table.Append([]string{s.ID, s.Name, s.FunctionID, s.Schedule, s.Status, s.NextRunAt})
		}
		_ = table.Render()
		return nil
	},
}

var deleteFnSchedCmd = &cobra.Command{
	Use:   "rm [id]",
	Short: "Delete a function schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createClient(opts)
		if err := client.DeleteFunctionSchedule(args[0]); err != nil {
			return err
		}
		fmt.Println("[SUCCESS] Schedule deleted")
		return nil
	},
}

var pauseFnSchedCmd = &cobra.Command{
	Use:   "pause [id]",
	Short: "Pause a function schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createClient(opts)
		if err := client.PauseFunctionSchedule(args[0]); err != nil {
			return err
		}
		fmt.Println("[SUCCESS] Schedule paused")
		return nil
	},
}

var resumeFnSchedCmd = &cobra.Command{
	Use:   "resume [id]",
	Short: "Resume a function schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createClient(opts)
		if err := client.ResumeFunctionSchedule(args[0]); err != nil {
			return err
		}
		fmt.Println("[SUCCESS] Schedule resumed")
		return nil
	},
}

var fnSchedLogsCmd = &cobra.Command{
	Use:   "logs [id]",
	Short: "Get runs for a function schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createClient(opts)
		runs, err := client.GetFunctionScheduleRuns(args[0])
		if err != nil {
			return err
		}

		if len(runs) == 0 {
			fmt.Println("No runs found.")
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "STATUS", "DURATION", "STARTED AT"})
		for _, r := range runs {
			_ = table.Append([]string{r.ID, r.Status, fmt.Sprintf("%dms", r.DurationMs), r.StartedAt})
		}
		_ = table.Render()
		return nil
	},
}

func init() {
	createFnSchedCmd.Flags().StringP("name", "n", "", "Schedule name")
	createFnSchedCmd.Flags().StringP("function", "f", "", "Function name or ID")
	createFnSchedCmd.Flags().StringP("schedule", "s", "", "Cron expression (e.g. '*/5 * * * *')")
	createFnSchedCmd.Flags().StringP("payload", "p", "{}", "Invocation payload (JSON)")
	_ = createFnSchedCmd.MarkFlagRequired("name")
	_ = createFnSchedCmd.MarkFlagRequired("function")
	_ = createFnSchedCmd.MarkFlagRequired("schedule")

	fnSchedCmd.AddCommand(createFnSchedCmd)
	fnSchedCmd.AddCommand(listFnSchedCmd)
	fnSchedCmd.AddCommand(deleteFnSchedCmd)
	fnSchedCmd.AddCommand(pauseFnSchedCmd)
	fnSchedCmd.AddCommand(resumeFnSchedCmd)
	fnSchedCmd.AddCommand(fnSchedLogsCmd)

	rootCmd.AddCommand(fnSchedCmd)
}
