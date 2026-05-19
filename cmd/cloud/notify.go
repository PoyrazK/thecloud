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

const notifyErrorFormat = "Error: %v\n"

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Manage CloudNotify (Pub/Sub)",
}

var createTopicCmd = &cobra.Command{
	Use:   "create-topic [name]",
	Short: "Create a new notification topic",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		topic, err := client.CreateTopic(args[0])
		if err != nil {
			fmt.Printf(notifyErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Topic created: %s (ID: %s)\n", topic.Name, topic.ID)
	},
}

var listTopicsCmd = &cobra.Command{
	Use:   "list-topics",
	Short: "List all notification topics",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		topics, err := client.ListTopics()
		if err != nil {
			fmt.Printf(notifyErrorFormat, err)
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

		client := createClient(opts)
		topicID := resolveTopicID(args[0], client)
		sub, err := client.Subscribe(topicID, protocol, endpoint)
		if err != nil {
			fmt.Printf(notifyErrorFormat, err)
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
		client := createClient(opts)
		topicID := resolveTopicID(args[0], client)
		err := client.Publish(topicID, args[1])
		if err != nil {
			fmt.Printf(notifyErrorFormat, err)
			return
		}

		fmt.Println("[SUCCESS] Message published")
	},
}

var unsubscribeCmd = &cobra.Command{
	Use:   "unsubscribe [subscription-id]",
	Short: "Unsubscribe from a topic",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		err := client.Unsubscribe(args[0])
		if err != nil {
			fmt.Printf(notifyErrorFormat, err)
			return
		}

		fmt.Println("[SUCCESS] Unsubscribed successfully")
	},
}

func init() {
	subscribeCmd.Flags().StringP("protocol", "p", "webhook", "Protocol (webhook/queue)")
	subscribeCmd.Flags().StringP("endpoint", "e", "", "Endpoint (URL or Queue ID)")
	cobra.CheckErr(subscribeCmd.MarkFlagRequired("endpoint"))

	notifyCmd.AddCommand(createTopicCmd)
	notifyCmd.AddCommand(listTopicsCmd)
	notifyCmd.AddCommand(subscribeCmd)
	notifyCmd.AddCommand(publishCmd)
	notifyCmd.AddCommand(unsubscribeCmd)
}

// resolveTopicID resolves a topic ID or name to a full UUID.
func resolveTopicID(idOrName string, client *sdk.Client) string {
	if _, err := uuid.Parse(idOrName); err == nil {
		return idOrName
	}
	topics, err := client.ListTopics()
	if err != nil {
		return idOrName
	}
	for _, t := range topics {
		if t.Name == idOrName {
			return t.ID
		}
	}
	return idOrName
}
