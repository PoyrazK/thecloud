// Package main provides the cloud CLI entrypoint.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const volumeErrorFormat = "Error: %v\n"

var volumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "Manage block storage volumes",
}

var volumeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all volumes",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		volumes, err := client.ListVolumes()
		if err != nil {
			fmt.Printf(volumeErrorFormat, err)
			return
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(volumes, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "SIZE", "STATUS", "ATTACHED TO"})

		for _, v := range volumes {
			id := v.ID.String()
			if len(id) > 8 {
				id = truncateID(id)
			}
			attachedTo := "-"
			if v.InstanceID != nil {
				attachedTo = truncateID(v.InstanceID.String())
			}
			table.Append([]string{
				id,
				v.Name,
				fmt.Sprintf("%d GB", v.SizeGB),
				v.Status,
				attachedTo,
			})
		}
		table.Render()
	},
}

var volumeCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new volume",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		size, _ := cmd.Flags().GetInt("size")

		client := createClient(opts)
		vol, err := client.CreateVolume(name, size)
		if err != nil {
			fmt.Printf(volumeErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Volume created!\n")
		data, _ := json.MarshalIndent(vol, "", "  ")
		fmt.Println(string(data))
	},
}

var volumeDeleteCmd = &cobra.Command{
	Use:   "rm [id/name]",
	Short: "Delete a volume",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.DeleteVolume(id); err != nil {
			fmt.Printf(volumeErrorFormat, err)
			return
		}
		fmt.Printf("[SUCCESS] Volume %s deleted.\n", id)
	},
}

var volumeResizeCmd = &cobra.Command{
	Use:   "resize [id/name] [sizeGB]",
	Short: "Resize a volume to a larger size",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		size, _ := strconv.Atoi(args[1])
		client := createClient(opts)
		if err := client.ResizeVolume(id, size); err != nil {
			fmt.Printf(volumeErrorFormat, err)
			return
		}
		fmt.Printf("[SUCCESS] Volume %s resized to %d GB.\n", id, size)
	},
}

func init() {
	volumeCmd.AddCommand(volumeListCmd)
	volumeCmd.AddCommand(volumeCreateCmd)
	volumeCmd.AddCommand(volumeDeleteCmd)
	volumeCmd.AddCommand(volumeResizeCmd)

	volumeCreateCmd.Flags().StringP("name", "n", "", "Name of the volume (required)")
	volumeCreateCmd.Flags().IntP("size", "s", 1, "Size in GB")
	cobra.CheckErr(volumeCreateCmd.MarkFlagRequired("name"))
}
