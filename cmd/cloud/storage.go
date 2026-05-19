// Package main provides the cloud CLI entrypoint.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

const (
	headerCreatedAt = "CREATED AT"
)

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Manage object storage",
}

var storageListCmd = &cobra.Command{
	Use:   "list [bucket]",
	Short: "List buckets or objects in a bucket",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)

		// List Buckets
		if len(args) == 0 {
			limit, _ := cmd.Flags().GetInt("limit")
			offset, _ := cmd.Flags().GetInt("offset")

			var buckets []sdk.Bucket
			var meta *sdk.ListResponse[sdk.Bucket]

			if limit > 0 || offset > 0 {
				var err error
				buckets, meta, err = client.ListBucketsWithPagination(limit, offset)
				if err != nil {
					fmt.Printf(errFmt, err)
					return
				}
			} else {
				var err error
				buckets, err = client.ListBuckets()
				if err != nil {
					fmt.Printf(errFmt, err)
					return
				}
			}

			if opts.JSON {
				if meta != nil {
					data, _ := json.MarshalIndent(meta, "", "  ")
					fmt.Println(string(data))
				} else {
					data, _ := json.MarshalIndent(buckets, "", "  ")
					fmt.Println(string(data))
				}
				return
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.Header([]string{"NAME", "PUBLIC", headerCreatedAt})

			for _, b := range buckets {
				table.Append([]string{
					b.Name,
					fmt.Sprintf("%v", b.IsPublic),
					b.CreatedAt.Format(time.RFC3339),
				})
			}
			table.Render()

			if meta != nil {
				fmt.Printf("\nShowing %d of %d total", len(buckets), meta.TotalCount)
				if meta.HasMore {
					fmt.Print(" (more available)")
				}
				fmt.Println()
			}
			return
		}

		// List Objects
		bucket := args[0]
		objects, err := client.ListObjects(bucket)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(objects, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"KEY", "SIZE", headerCreatedAt, "ARN"})

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

		f, err := os.Open(filepath.Clean(filePath))
		if err != nil {
			fmt.Printf("Error opening file: %v\n", err)
			return
		}
		defer func() { _ = f.Close() }()

		client := createClient(opts)
		obj, err := client.UploadObject(bucket, key, f)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		fmt.Printf("[SUCCESS] Uploaded %s to bucket %s (VersionID: %s)\n", key, bucket, obj.VersionID)
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

		versionID, _ := cmd.Flags().GetString("version")

		client := createClient(opts)
		var body io.ReadCloser
		var err error
		if versionID != "" {
			body, err = client.DownloadObject(bucket, key, versionID)
		} else {
			body, err = client.DownloadObject(bucket, key)
		}
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}
		defer func() { _ = body.Close() }()

		out, err := os.Create(filepath.Clean(dest))
		if err != nil {
			fmt.Printf("Error creating destination file: %v\n", err)
			return
		}
		defer func() { _ = out.Close() }()

		_, err = io.Copy(out, body)
		if err != nil {
			fmt.Printf("Error writing file: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Downloaded %s to %s\n", key, dest)
	},
}

var storageDeleteCmd = &cobra.Command{
	Use:     "rm [bucket] [key]",
	Short:   "Delete an object from a bucket",
	Aliases: []string{"delete"},
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		bucket := args[0]
		key := args[1]

		versionID, _ := cmd.Flags().GetString("version")

		client := createClient(opts)
		var err error
		if versionID != "" {
			err = client.DeleteObject(bucket, key, versionID)
		} else {
			err = client.DeleteObject(bucket, key)
		}

		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		msg := fmt.Sprintf("Deleted %s from bucket %s", key, bucket)
		if versionID != "" {
			msg += fmt.Sprintf(" (Version: %s)", versionID)
		}
		fmt.Printf("[SUCCESS] %s\n", msg)
	},
}

var createBucketCmd = &cobra.Command{
	Use:     "mb [name]",
	Aliases: []string{"create-bucket", "create"},
	Short:   "Create a new storage bucket",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		public, _ := cmd.Flags().GetBool("public")

		client := createClient(opts)
		bucket, err := client.CreateBucket(name, public)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(bucket, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Printf("[SUCCESS] Created bucket %s (Public: %v)\n", bucket.Name, bucket.IsPublic)
	},
}

var deleteBucketCmd = &cobra.Command{
	Use:     "rb [name]",
	Aliases: []string{"delete-bucket", "rm", "delete"},
	Short:   "Delete a storage bucket",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		force, _ := cmd.Flags().GetBool("force")

		client := createClient(opts)
		if err := client.DeleteBucket(cmd.Context(), name, force); err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		fmt.Printf("[SUCCESS] Deleted bucket %s\n", name)
	},
}

var storageClusterStatusCmd = &cobra.Command{
	Use:   "cluster-status",
	Short: "Get storage cluster status",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		status, err := client.GetStorageClusterStatus()
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(status, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"NODE ID", "ADDRESS", "STATUS", "LAST SEEN"})

		for _, n := range status.Nodes {
			table.Append([]string{
				n.ID,
				n.Address,
				n.Status,
				n.LastSeen.Format(time.RFC3339),
			})
		}
		table.Render()
	},
}

func init() {
	storageCmd.AddCommand(storageListCmd)
	storageCmd.AddCommand(storageUploadCmd)
	storageCmd.AddCommand(storageDownloadCmd)
	storageCmd.AddCommand(storageDeleteCmd)
	storageCmd.AddCommand(storageVersionsCmd)
	storageCmd.AddCommand(storageVersioningCmd)
	storageCmd.AddCommand(createBucketCmd)
	storageCmd.AddCommand(deleteBucketCmd)
	storageCmd.AddCommand(storageClusterStatusCmd)
	storageCmd.AddCommand(storagePresignCmd)

	createBucketCmd.Flags().Bool("public", false, "Make bucket public")
	deleteBucketCmd.Flags().BoolP("force", "f", false, "Delete bucket even if not empty")

	storageUploadCmd.Flags().String("key", "", "Custom key for the object")
	storageDownloadCmd.Flags().String("version", "", "Specific version to download")
	storageDeleteCmd.Flags().String("version", "", "Specific version to delete")
	storagePresignCmd.Flags().String("method", "GET", "HTTP method (GET or PUT)")
	storagePresignCmd.Flags().Int("expires", 900, "Expiration in seconds (default 15 mins)")

	storageListCmd.Flags().Int("limit", 0, "Maximum number of results (0 = use server default)")
	storageListCmd.Flags().Int("offset", 0, "Number of results to skip")
}

var storageVersionsCmd = &cobra.Command{
	Use:   "versions [bucket] [key]",
	Short: "List all versions of an object",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		bucket := args[0]
		key := args[1]

		client := createClient(opts)
		versions, err := client.ListVersions(bucket, key)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(versions, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"VERSION ID", "LATEST", "SIZE", headerCreatedAt})

		for _, v := range versions {
			table.Append([]string{
				v.VersionID,
				fmt.Sprintf("%v", v.IsLatest),
				fmt.Sprintf("%d", v.SizeBytes),
				v.CreatedAt.Format(time.RFC3339),
			})
		}
		table.Render()
	},
}

var storageVersioningCmd = &cobra.Command{
	Use:   "versioning [bucket] [status]",
	Short: "Enable or disable versioning for a bucket",
	Long:  "status can be 'on' or 'off'",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		bucket := args[0]
		status := args[1]

		enabled := false
		switch status {
		case "on", "enabled", "true":
			enabled = true
		case "off", "disabled", "false":
			enabled = false
		default:
			fmt.Printf("Invalid status: %s. Use 'on' or 'off'.\n", status)
			return
		}

		client := createClient(opts)
		if err := client.SetBucketVersioning(bucket, enabled); err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		fmt.Printf("[SUCCESS] Versioning for bucket %s is now %s\n", bucket, status)
	},
}

var storagePresignCmd = &cobra.Command{
	Use:   "presign [bucket] [key]",
	Short: "Generate a pre-signed URL for an object",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		bucket := args[0]
		key := args[1]
		method, _ := cmd.Flags().GetString("method")
		expires, _ := cmd.Flags().GetInt("expires")

		client := createClient(opts)
		url, err := client.GeneratePresignedURL(bucket, key, method, expires)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(url, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Printf("URL: %s\n", url.URL)
		fmt.Printf("Expires: %s\n", url.ExpiresAt.Format(time.RFC3339))
		fmt.Printf("Method: %s\n", url.Method)
	},
}
