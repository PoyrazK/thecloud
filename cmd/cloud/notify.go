package main

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Manage CloudNotify (Pub/Sub)",
}

var createTopicCmd = &cobra.Command{
	Use:   "create-topic [name]",
	Short: "Create a new notification topic",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		topic, err := client.CreateTopic(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Topic created: %s (ID: %s)\n", topic.Name, topic.ID)
	},
}

var listTopicsCmd = &cobra.Command{
	Use:   "list-topics",
	Short: "List all notification topics",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		topics, err := client.ListTopics()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "ARN"})
		for _, t := range topics {
			table.Append([]string{t.ID, t.Name, t.ARN})
		}
		table.Render()
	},
}

var subscribeCmd = &cobra.Command{
	Use:   "subscribe [topic-id]",
	Short: "Subscribe to a topic",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		protocol, _ := cmd.Flags().GetString("protocol")
		endpoint, _ := cmd.Flags().GetString("endpoint")

		client := getClient()
		sub, err := client.Subscribe(args[0], protocol, endpoint)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Subscription created (ID: %s)\n", sub.ID)
	},
}

var publishCmd = &cobra.Command{
	Use:   "publish [topic-id] [message]",
	Short: "Publish a message to a topic",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		err := client.Publish(args[0], args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Println("[SUCCESS] Message published")
	},
}

func init() {
	subscribeCmd.Flags().StringP("protocol", "p", "webhook", "Protocol (webhook/queue)")
	subscribeCmd.Flags().StringP("endpoint", "e", "", "Endpoint (URL or Queue ID)")
	subscribeCmd.MarkFlagRequired("endpoint")

	notifyCmd.AddCommand(createTopicCmd)
	notifyCmd.AddCommand(listTopicsCmd)
	notifyCmd.AddCommand(subscribeCmd)
	notifyCmd.AddCommand(publishCmd)

	rootCmd.AddCommand(notifyCmd)
}
