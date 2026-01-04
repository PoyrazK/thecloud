package main

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
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

		client := getClient()
		job, err := client.CreateCronJob(args[0], args[1], args[2], method, payload)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
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
		client := getClient()
		jobs, err := client.ListCronJobs()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
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
		client := getClient()
		err := client.PauseCronJob(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
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
		client := getClient()
		err := client.ResumeCronJob(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
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
		client := getClient()
		err := client.DeleteCronJob(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("[SUCCESS] Job deleted")
	},
}

func init() {
	createCronCmd.Flags().StringP("method", "X", "POST", "HTTP method")
	createCronCmd.Flags().StringP("payload", "d", "", "Request payload")

	cronCmd.AddCommand(createCronCmd)
	cronCmd.AddCommand(listCronCmd)
	cronCmd.AddCommand(pauseCronCmd)
	cronCmd.AddCommand(resumeCronCmd)
	cronCmd.AddCommand(deleteCronCmd)

	rootCmd.AddCommand(cronCmd)
}
