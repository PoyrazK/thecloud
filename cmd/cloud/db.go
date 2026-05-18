// Package main provides the cloud CLI entrypoint.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const (
	errorFormat = "Error: %v\n"
	detailRow   = "%-15s %v\n"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Manage managed database instances (RDS)",
}

var dbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all database instances",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		databases, err := client.ListDatabases()
		if err != nil {
			fmt.Printf(errorFormat, err)
			return
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(databases, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "ENGINE", "VERSION", "STATUS", "PORT"})

		for _, db := range databases {
			id := truncateID(db.ID)

			if err := table.Append([]string{
				id,
				db.Name,
				db.Engine,
				db.Version,
				db.Status,
				fmt.Sprintf("%d", db.Port),
			}); err != nil {
				fmt.Printf(errorFormat, err)
				return
			}
		}
		if err := table.Render(); err != nil {
			fmt.Printf(errorFormat, err)
			return
		}
	},
}

var dbCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new managed database instance",
	Run: func(cmd *cobra.Command, _ []string) {
		name, _ := cmd.Flags().GetString("name")
		engine, _ := cmd.Flags().GetString("engine")
		version, _ := cmd.Flags().GetString("version")
		vpc, _ := cmd.Flags().GetString("vpc")
		size, _ := cmd.Flags().GetInt("size")

		if size < 10 {
			fmt.Printf(errorFormat, "--size must be at least 10GB")
			return
		}

		var vpcPtr *string
		if vpc != "" {
			vpcPtr = &vpc
		}

		client := createClient(opts)
		db, err := client.CreateDatabase(name, engine, version, vpcPtr, size)
		if err != nil {
			fmt.Printf(errorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Database %s created successfully!\n", name)
		if opts.JSON {
			// Mask password for JSON output
			dbCopy := *db
			dbCopy.Password = "********"
			data, _ := json.MarshalIndent(dbCopy, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("ID:       %s\n", db.ID)
			fmt.Printf("Username: %s\n", db.Username)
			fmt.Println("\n SECURITY WARNING: The following password is shown ONCE only.")
			fmt.Println("    Save it securely - it cannot be retrieved later.")

			// Only display password if we are likely in a terminal or explicitly allowed
			if os.Getenv("HIDE_SENSITIVE") != "true" {
				fmt.Printf("Password: %s\n", db.Password)
			} else {
				fmt.Println("Password: [HIDDEN by HIDE_SENSITIVE environment variable]")
			}
			fmt.Println("\nFor security reasons, this password will not be displayed again.")
		}
	},
}

var dbShowCmd = &cobra.Command{
	Use:   "show [id/name]",
	Short: "Show detailed database information",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		db, err := client.GetDatabase(id)
		if err != nil {
			fmt.Printf(errorFormat, err)
			return
		}

		fmt.Printf("\nDatabase Details\n")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf(detailRow, "ID:", db.ID)
		fmt.Printf(detailRow, "Name:", db.Name)
		fmt.Printf(detailRow, "Engine:", db.Engine)
		fmt.Printf(detailRow, "Version:", db.Version)
		fmt.Printf(detailRow, "Status:", db.Status)
		fmt.Printf(detailRow, "Port:", db.Port)
		fmt.Printf(detailRow, "Username:", db.Username)
		fmt.Printf(detailRow, "VPC ID:", db.VpcID)
		fmt.Printf(detailRow, "Created At:", db.CreatedAt)
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println("")
	},
}

var dbRmCmd = &cobra.Command{
	Use:   "rm [id]",
	Short: "Remove a database instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.DeleteDatabase(id); err != nil {
			fmt.Printf(errorFormat, err)
			return
		}
		fmt.Println("[SUCCESS] Database removed.")
	},
}

var dbConnCmd = &cobra.Command{
	Use:   "connection [id]",
	Short: "Get database connection string",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		connStr, err := client.GetDatabaseConnectionString(id)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Connection String: %s\n", connStr)
	},
}

var dbRotateCmd = &cobra.Command{
	Use:   "rotate-credentials [id]",
	Short: "Rotate database user credentials",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.RotateDatabaseCredentials(id); err != nil {
			fmt.Printf(errorFormat, err)
			return
		}
		fmt.Println("[SUCCESS] Database credentials rotated successfully.")
	},
}

var dbResizeCmd = &cobra.Command{
	Use:   "resize [id] [sizeGB]",
	Short: "Resize database allocated storage",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		size, _ := strconv.Atoi(args[1])
		client := createClient(opts)
		if err := client.ResizeDatabase(id, size); err != nil {
			fmt.Printf(errorFormat, err)
			return
		}
		fmt.Printf("[SUCCESS] Database %s resized to %d GB.\n", id, size)
	},
}

func init() {
	dbCmd.AddCommand(dbListCmd)
	dbCmd.AddCommand(dbCreateCmd)
	dbCmd.AddCommand(dbShowCmd)
	dbCmd.AddCommand(dbRmCmd)
	dbCmd.AddCommand(dbConnCmd)
	dbCmd.AddCommand(dbRotateCmd)
	dbCmd.AddCommand(dbResizeCmd)

	dbCreateCmd.Flags().StringP("name", "n", "", "Name of the database (required)")
	dbCreateCmd.Flags().StringP("engine", "e", "postgres", "Database engine (postgres/mysql)")
	dbCreateCmd.Flags().StringP("version", "v", "16", "Engine version")
	dbCreateCmd.Flags().StringP("vpc", "V", "", "VPC ID to attach to")
	dbCreateCmd.Flags().Int("size", 10, "Allocated storage in GB (minimum 10GB)")
	_ = dbCreateCmd.MarkFlagRequired("name")
}
