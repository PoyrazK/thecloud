// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage CloudCron (Scheduled Tasks)",
}

var createCronCmd = &cobra.Command{
	Use:   "create [name] [schedule] [url]",
	Short: "Create a new scheduled task",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		method, _ := cmd.Flags().GetString("method")
		payload, _ := cmd.Flags().GetString("payload")

		client := createClient(opts)
		job, err := client.CreateCronJob(args[0], args[1], args[2], method, payload)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		fmt.Printf("[SUCCESS] Cron job created: %s (ID: %s)\n", job.Name, job.ID)
		fmt.Printf("Next run: %s\n", job.NextRunAt)
	},
}

var listCronCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scheduled tasks",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		jobs, err := client.ListCronJobs()
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "SCHEDULE", "STATUS", "NEXT RUN"})
		for _, j := range jobs {
			table.Append([]string{j.ID, j.Name, j.Schedule, j.Status, j.NextRunAt})
		}
		table.Render()
	},
}

var pauseCronCmd = &cobra.Command{
	Use:   "pause [id]",
	Short: "Pause a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		jobID := resolveCronJobID(args[0], client)
		err := client.PauseCronJob(jobID)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}
		fmt.Println("[SUCCESS] Job paused")
	},
}

var resumeCronCmd = &cobra.Command{
	Use:   "resume [id]",
	Short: "Resume a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		jobID := resolveCronJobID(args[0], client)
		err := client.ResumeCronJob(jobID)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}
		fmt.Println("[SUCCESS] Job resumed")
	},
}

var deleteCronCmd = &cobra.Command{
	Use:   "rm [id]",
	Short: "Delete a scheduled task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		jobID := resolveCronJobID(args[0], client)
		err := client.DeleteCronJob(jobID)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}
		fmt.Println("[SUCCESS] Job deleted")
	},
}

var logsCronCmd = &cobra.Command{
	Use:   "logs [id]",
	Short: "View execution history of a cron job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		jobID := resolveCronJobID(args[0], client)
		runs, err := client.GetCronJobRuns(jobID, 50)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if len(runs) == 0 {
			fmt.Println("No runs found")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "STATUS", "CODE", "DURATION", "STARTED AT"})
		for _, r := range runs {
			table.Append([]string{truncateID(r.ID), r.Status, fmt.Sprintf("%d", r.StatusCode), fmt.Sprintf("%dms", r.DurationMs), r.StartedAt})
		}
		table.Render()
	},
}

var updateCronCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update a cron job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		schedule, _ := cmd.Flags().GetString("schedule")
		url, _ := cmd.Flags().GetString("url")
		method, _ := cmd.Flags().GetString("method")
		payload, _ := cmd.Flags().GetString("payload")

		job := &sdk.CronJob{
			Name:          name,
			Schedule:      schedule,
			TargetURL:     url,
			TargetMethod:  method,
			TargetPayload: payload,
		}

		client := createClient(opts)
		jobID := resolveCronJobID(args[0], client)
		updated, err := client.UpdateCronJob(jobID, job)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}
		fmt.Printf("[SUCCESS] Cron job %s updated (ID: %s)\n", updated.Name, updated.ID)
	},
}

func init() {
	createCronCmd.Flags().StringP("method", "X", "POST", "HTTP method")
	createCronCmd.Flags().StringP("payload", "d", "", "Request payload")

	updateCronCmd.Flags().StringP("name", "n", "", "Name")
	updateCronCmd.Flags().StringP("schedule", "s", "", "Cron schedule")
	updateCronCmd.Flags().StringP("url", "u", "", "Target URL")
	updateCronCmd.Flags().StringP("method", "X", "POST", "HTTP method")
	updateCronCmd.Flags().StringP("payload", "d", "", "Request payload")

	cronCmd.AddCommand(createCronCmd)
	cronCmd.AddCommand(listCronCmd)
	cronCmd.AddCommand(pauseCronCmd)
	cronCmd.AddCommand(resumeCronCmd)
	cronCmd.AddCommand(deleteCronCmd)
	cronCmd.AddCommand(logsCronCmd)
	cronCmd.AddCommand(updateCronCmd)
}

// resolveCronJobID resolves a cron job ID or name to a full UUID.
func resolveCronJobID(idOrName string, client *sdk.Client) string {
	if _, err := uuid.Parse(idOrName); err == nil {
		return idOrName
	}
	jobs, err := client.ListCronJobs()
	if err != nil {
		return idOrName
	}
	for _, j := range jobs {
		if j.Name == idOrName {
			return j.ID
		}
	}
	return idOrName
}
