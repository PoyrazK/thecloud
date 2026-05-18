package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/google/uuid"
	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/spf13/cobra"
)

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Manage DNS zones and records",
}

const dnsErrorFormat = "Error: %v\n"

var dnsListZonesCmd = &cobra.Command{
	Use:   "list-zones",
	Short: "List all DNS zones",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		zones, err := client.ListDNSZones()
		if err != nil {
			fmt.Printf(dnsErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(zones)
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "VPC ID", "CREATED AT"})

		for _, z := range zones {
			vpcID := "Public"
			if z.VpcID != uuid.Nil {
				vpcID = truncateID(z.VpcID.String())
			}
			table.Append([]string{
				truncateID(z.ID.String()),
				z.Name,
				vpcID,
				z.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		table.Render()
	},
}

var dnsCreateZoneCmd = &cobra.Command{
	Use:   "create-zone [name]",
	Short: "Create a new DNS zone",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		desc, _ := cmd.Flags().GetString("description")
		vpcIDOrName, _ := cmd.Flags().GetString("vpc-id")

		var vpcPtr *string
		if vpcIDOrName != "" {
			vpcPtr = &vpcIDOrName
		}

		client := createClient(opts)
		zone, err := client.CreateDNSZone(name, desc, vpcPtr)
		if err != nil {
			fmt.Printf(dnsErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(zone)
		} else {
			fmt.Printf("[SUCCESS] DNS Zone %s created successfully!\n", zone.Name)
			fmt.Printf("ID: %s\n", zone.ID)
		}
	},
}

var dnsDeleteZoneCmd = &cobra.Command{
	Use:     "rm-zone [id]",
	Aliases: []string{"delete-zone"},
	Short:   "Delete a DNS zone",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.DeleteDNSZone(id); err != nil {
			fmt.Printf(dnsErrorFormat, err)
			return
		}
		fmt.Printf("[SUCCESS] DNS Zone %s deleted.\n", id)
	},
}

var dnsListRecordsCmd = &cobra.Command{
	Use:   "list-records [zone-id]",
	Short: "List all records in a DNS zone",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		zoneID := args[0]
		client := createClient(opts)
		records, err := client.ListDNSRecords(zoneID)
		if err != nil {
			fmt.Printf(dnsErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(records)
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "TYPE", "CONTENT", "TTL", "PRIO", "AUTO"})

		for _, r := range records {
			prio := "-"
			if r.Priority != nil {
				prio = strconv.Itoa(*r.Priority)
			}
			auto := "No"
			if r.AutoManaged {
				auto = "Yes"
			}
			table.Append([]string{
				truncateID(r.ID.String()),
				r.Name,
				string(r.Type),
				r.Content,
				strconv.Itoa(r.TTL),
				prio,
				auto,
			})
		}
		table.Render()
	},
}

var dnsCreateRecordCmd = &cobra.Command{
	Use:   "create-record [zone-id]",
	Short: "Create a new DNS record",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		zoneIDStr := args[0]
		zoneID, err := uuid.Parse(zoneIDStr)
		if err != nil {
			fmt.Printf("Error: invalid zone-id: %v\n", err)
			return
		}

		name, _ := cmd.Flags().GetString("name")
		recordType, _ := cmd.Flags().GetString("type")
		content, _ := cmd.Flags().GetString("content")
		ttl, _ := cmd.Flags().GetInt("ttl")
		priority, _ := cmd.Flags().GetInt("priority")

		var prioPtr *int
		if cmd.Flags().Changed("priority") {
			prioPtr = &priority
		}

		client := createClient(opts)
		record, err := client.CreateDNSRecord(zoneID, name, domain.RecordType(recordType), content, ttl, prioPtr)
		if err != nil {
			fmt.Printf(dnsErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(record)
		} else {
			fmt.Printf("[SUCCESS] DNS Record %s (%s) created!\n", record.Name, record.Type)
			fmt.Printf("ID: %s\n", record.ID)
		}
	},
}

var dnsDeleteRecordCmd = &cobra.Command{
	Use:     "rm-record [id]",
	Aliases: []string{"delete-record"},
	Short:   "Delete a DNS record",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.DeleteDNSRecord(id); err != nil {
			fmt.Printf(dnsErrorFormat, err)
			return
		}
		fmt.Printf("[SUCCESS] DNS Record %s deleted.\n", id)
	},
}

func init() {
	dnsCreateZoneCmd.Flags().String("description", "", "Description of the zone")
	dnsCreateZoneCmd.Flags().String("vpc-id", "", "VPC ID for private DNS (required)")
	_ = dnsCreateZoneCmd.MarkFlagRequired("vpc-id")

	dnsCreateRecordCmd.Flags().String("name", "", "Record name (e.g., 'www')")
	dnsCreateRecordCmd.Flags().String("type", "A", "Record type (A, AAAA, CNAME, MX, TXT)")
	dnsCreateRecordCmd.Flags().String("content", "", "Record content (e.g., IP address)")
	dnsCreateRecordCmd.Flags().Int("ttl", 3600, "Time To Live in seconds")
	dnsCreateRecordCmd.Flags().Int("priority", 0, "Priority for MX records")

	_ = dnsCreateRecordCmd.MarkFlagRequired("name")
	_ = dnsCreateRecordCmd.MarkFlagRequired("content")

	dnsCmd.AddCommand(dnsListZonesCmd)
	dnsCmd.AddCommand(dnsCreateZoneCmd)
	dnsCmd.AddCommand(dnsDeleteZoneCmd)
	dnsCmd.AddCommand(dnsListRecordsCmd)
	dnsCmd.AddCommand(dnsCreateRecordCmd)
	dnsCmd.AddCommand(dnsDeleteRecordCmd)
}
