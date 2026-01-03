package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Manage message queues",
}

var listQueuesCmd = &cobra.Command{
	Use:   "list",
	Short: "List all queues",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		queues, err := client.ListQueues()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if outputJSON {
			data, _ := json.MarshalIndent(queues, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "STATUS", "ARN"})

		for _, q := range queues {
			table.Append([]string{
				q.ID[:8],
				q.Name,
				string(q.Status),
				q.ARN,
			})
		}
		table.Render()
	},
}

var createQueueCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new message queue",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		vt, _ := cmd.Flags().GetInt("visibility-timeout")
		rd, _ := cmd.Flags().GetInt("retention-days")
		ms, _ := cmd.Flags().GetInt("max-message-size")

		client := getClient()
		q, err := client.CreateQueue(name, &vt, &rd, &ms)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Queue created successfully!\n")
		data, _ := json.MarshalIndent(q, "", "  ")
		fmt.Println(string(data))
	},
}

var deleteQueueCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a message queue",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := getClient()
		if err := client.DeleteQueue(id); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("[SUCCESS] Queue deleted successfully.")
	},
}

var sendMessageCmd = &cobra.Command{
	Use:   "send [queue-id] [message]",
	Short: "Send a message to a queue",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		body := args[1]
		client := getClient()
		msg, err := client.SendMessage(id, body)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Message sent! ID: %s\n", msg.ID)
	},
}

var receiveMessagesCmd = &cobra.Command{
	Use:   "receive [queue-id]",
	Short: "Receive messages from a queue",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		max, _ := cmd.Flags().GetInt("max")
		client := getClient()
		msgs, err := client.ReceiveMessages(id, max)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if len(msgs) == 0 {
			fmt.Println("No messages available.")
			return
		}

		if outputJSON {
			data, _ := json.MarshalIndent(msgs, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "BODY", "RECEIPT HANDLE"})

		for _, m := range msgs {
			table.Append([]string{
				m.ID[:8],
				m.Body,
				m.ReceiptHandle,
			})
		}
		table.Render()
	},
}

var ackMessageCmd = &cobra.Command{
	Use:   "ack [queue-id] [receipt-handle]",
	Short: "Acknowledge (delete) a message from a queue",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		handle := args[1]
		client := getClient()
		if err := client.DeleteMessage(id, handle); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("[SUCCESS] Message acknowledged and deleted.")
	},
}

var purgeQueueCmd = &cobra.Command{
	Use:   "purge [queue-id]",
	Short: "Delete all messages from a queue",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := getClient()
		if err := client.PurgeQueue(id); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("[SUCCESS] Queue purged.")
	},
}

func init() {
	queueCmd.AddCommand(listQueuesCmd)
	queueCmd.AddCommand(createQueueCmd)
	queueCmd.AddCommand(deleteQueueCmd)
	queueCmd.AddCommand(sendMessageCmd)
	queueCmd.AddCommand(receiveMessagesCmd)
	queueCmd.AddCommand(ackMessageCmd)
	queueCmd.AddCommand(purgeQueueCmd)

	createQueueCmd.Flags().Int("visibility-timeout", 30, "Visibility timeout in seconds")
	createQueueCmd.Flags().Int("retention-days", 4, "Retention period in days")
	createQueueCmd.Flags().Int("max-message-size", 262144, "Max message size in bytes")

	receiveMessagesCmd.Flags().Int("max", 1, "Maximum number of messages to receive")

	rootCmd.AddCommand(queueCmd)
}
