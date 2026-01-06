package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage volume snapshots",
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all snapshots",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		snapshots, err := client.ListSnapshots()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if outputJSON {
			data, _ := json.MarshalIndent(snapshots, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "VOLUME", "SIZE", "STATUS", "CREATED AT"})

		for _, s := range snapshots {
			id := s.ID.String()
			if len(id) > 8 {
				id = id[:8]
			}
			vol := s.VolumeName
			if vol == "" {
				vol = s.VolumeID.String()[:8]
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
	Use:   "create [volume-id]",
	Short: "Create a snapshot from a volume",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		volID, err := uuid.Parse(args[0])
		if err != nil {
			fmt.Printf("Error: invalid volume ID: %v\n", err)
			return
		}
		desc, _ := cmd.Flags().GetString("desc")

		client := getClient()
		snapshot, err := client.CreateSnapshot(volID, desc)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Snapshot creation started!\n")
		data, _ := json.MarshalIndent(snapshot, "", "  ")
		fmt.Println(string(data))
	},
}

var snapshotDeleteCmd = &cobra.Command{
	Use:   "rm [id]",
	Short: "Delete a snapshot",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := getClient()
		if err := client.DeleteSnapshot(id); err != nil {
			fmt.Printf("Error: %v\n", err)
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

		client := getClient()
		vol, err := client.RestoreSnapshot(id, name)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Snapshot restored to volume %s!\n", vol.Name)
		data, _ := json.MarshalIndent(vol, "", "  ")
		fmt.Println(string(data))
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotCreateCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)

	snapshotCreateCmd.Flags().StringP("desc", "d", "", "Description of the snapshot")
	snapshotRestoreCmd.Flags().StringP("name", "n", "", "Name of the new volume (required)")
	snapshotRestoreCmd.MarkFlagRequired("name")
}
