package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Manage object storage",
}

var storageListCmd = &cobra.Command{
	Use:   "list [bucket]",
	Short: "List objects in a bucket",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bucket := args[0]
		client := getClient()
		objects, err := client.ListObjects(bucket)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if outputJSON {
			data, _ := json.MarshalIndent(objects, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"KEY", "SIZE", "CREATED AT", "ARN"})

		for _, obj := range objects {
			table.Append([]string{
				obj.Key,
				fmt.Sprintf("%d", obj.SizeBytes),
				obj.CreatedAt.Format(time.RFC3339),
				obj.ARN,
			})
		}
		table.Render()
	},
}

var storageUploadCmd = &cobra.Command{
	Use:   "upload [bucket] [file]",
	Short: "Upload a file to a bucket",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		bucket := args[0]
		filePath := args[1]
		key := cmd.Flag("key").Value.String()
		if key == "" {
			key = filePath // Default key to filename
		}

		f, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("Error opening file: %v\n", err)
			return
		}
		defer f.Close()

		client := getClient()
		if err := client.UploadObject(bucket, key, f); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Uploaded %s to bucket %s\n", key, bucket)
	},
}

var storageDownloadCmd = &cobra.Command{
	Use:   "download [bucket] [key] [dest]",
	Short: "Download an object from a bucket",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		bucket := args[0]
		key := args[1]
		dest := args[2]

		client := getClient()
		body, err := client.DownloadObject(bucket, key)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer body.Close()

		out, err := os.Create(dest)
		if err != nil {
			fmt.Printf("Error creating destination file: %v\n", err)
			return
		}
		defer out.Close()

		_, err = io.Copy(out, body)
		if err != nil {
			fmt.Printf("Error writing file: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Downloaded %s to %s\n", key, dest)
	},
}

var storageDeleteCmd = &cobra.Command{
	Use:   "delete [bucket] [key]",
	Short: "Delete an object from a bucket",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		bucket := args[0]
		key := args[1]

		client := getClient()
		if err := client.DeleteObject(bucket, key); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Deleted %s from bucket %s\n", key, bucket)
	},
}

func init() {
	storageCmd.AddCommand(storageListCmd)
	storageCmd.AddCommand(storageUploadCmd)
	storageCmd.AddCommand(storageDownloadCmd)
	storageCmd.AddCommand(storageDeleteCmd)

	storageUploadCmd.Flags().String("key", "", "Custom key for the object")
}
