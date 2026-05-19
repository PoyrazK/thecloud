// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const snapshotErrorFormat = "Error: %v\n"

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage volume snapshots",
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all snapshots",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		snapshots, err := client.ListSnapshots()
		if err != nil {
			fmt.Printf(snapshotErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(snapshots)
			return
		}

		if len(snapshots) == 0 {
			fmt.Println("No snapshots found.")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "VOLUME", "SIZE", "STATUS", "CREATED AT"})

		for _, s := range snapshots {
			id := truncateID(s.ID.String())
			vol := s.VolumeName
			if vol == "" {
				vol = truncateID(s.VolumeID.String())
			}
			table.Append([]string{
				id,
				vol,
				fmt.Sprintf("%d GB", s.SizeGB),
				string(s.Status),
				s.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
		table.Render()
	},
}

var snapshotCreateCmd = &cobra.Command{
	Use:   "create [volume-id|volume-name]",
	Short: "Create a snapshot from a volume",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		volIDOrName := args[0]
		desc, err := cmd.Flags().GetString("desc")
		if err != nil {
			fmt.Printf(snapshotErrorFormat, err)
			return
		}

		client := createClient(opts)
		snapshot, err := client.CreateSnapshot(cmd.Context(), volIDOrName, desc)
		if err != nil {
			fmt.Printf(snapshotErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(snapshot)
		} else {
			fmt.Printf("[SUCCESS] Snapshot creation started!\n")
			fmt.Printf("ID: %s\n", snapshot.ID)
			fmt.Printf("Status: %s\n", snapshot.Status)
		}
	},
}

var snapshotDeleteCmd = &cobra.Command{
	Use:     "rm [id]",
	Aliases: []string{"delete"},
	Short:   "Delete a snapshot",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.DeleteSnapshot(id); err != nil {
			fmt.Printf(snapshotErrorFormat, err)
			return
		}
		fmt.Printf("[SUCCESS] Snapshot %s deleted.\n", id)
	},
}

var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore [id]",
	Short: "Restore a snapshot to a new volume",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		name, _ := cmd.Flags().GetString("name")

		client := createClient(opts)
		vol, err := client.RestoreSnapshot(id, name)
		if err != nil {
			fmt.Printf(snapshotErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(vol)
		} else {
			fmt.Printf("[SUCCESS] Snapshot restored to volume %s!\n", vol.Name)
			fmt.Printf("Volume ID: %s\n", vol.ID)
		}
	},
}

func init() {
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotCreateCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)

	snapshotCreateCmd.Flags().StringP("desc", "d", "", "Description of the snapshot")
	snapshotRestoreCmd.Flags().StringP("name", "n", "", "Name of the new volume (required)")
	cobra.CheckErr(snapshotRestoreCmd.MarkFlagRequired("name"))
}
