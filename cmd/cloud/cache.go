// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage cache instances (Redis)",
}

var createCacheCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new cache instance",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		version, _ := cmd.Flags().GetString("version")
		memory, _ := cmd.Flags().GetInt("memory")
		vpcID, _ := cmd.Flags().GetString("vpc")
		wait, _ := cmd.Flags().GetBool("wait")

		client := createClient(opts)
		var vpcPtr *string
		if vpcID != "" {
			vpcPtr = &vpcID
		}

		fmt.Printf("Creating Redis cache '%s' (v%s, %dMB)...\n", name, version, memory)
		cache, err := client.CreateCache(name, version, memory, vpcPtr)
		if err != nil {
			fmt.Printf("Error creating cache: %v\n", err)
			return
		}

		fmt.Printf("Cache created with ID: %s\n", cache.ID)

		if wait {
			fmt.Print("Waiting for cache to be RUNNING...")
			for i := 0; i < 30; i++ {
				c, err := client.GetCache(cache.ID)
				if err == nil && c.Status == "RUNNING" {
					fmt.Println("\nCache is now RUNNING!")
					return
				}
				fmt.Print(".")
				time.Sleep(1 * time.Second)
			}
			fmt.Println("\nTimeout waiting for cache to be ready.")
		}
	},
}

var listCacheCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cache instances",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		caches, err := client.ListCaches()
		if err != nil {
			fmt.Printf("Error listing caches: %v\n", err)
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tNAME\tSTATUS\tPORT\tVERSION\tMEMORY")
		for _, c := range caches {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%dMB\n", c.ID, c.Name, c.Status, c.Port, c.Version, c.MemoryMB)
		}
		_ = w.Flush()
	},
}

var getCacheCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show details of a cache instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		cacheID := resolveCacheID(args[0], client)
		cache, err := client.GetCache(cacheID)
		if err != nil {
			fmt.Printf("Error getting cache: %v\n", err)
			return
		}

		fmt.Printf("ID:        %s\n", cache.ID)
		fmt.Printf("Name:      %s\n", cache.Name)
		fmt.Printf("Engine:    %s\n", cache.Engine)
		fmt.Printf("Version:   %s\n", cache.Version)
		fmt.Printf("Status:    %s\n", cache.Status)
		fmt.Printf("Port:      %d\n", cache.Port)
		fmt.Printf("Memory:    %d MB\n", cache.MemoryMB)
		fmt.Printf("Password:  %s\n", "******** (use 'cache connection' or check secrets)")
		if cache.VpcID != nil {
			fmt.Printf("VPC ID:    %s\n", *cache.VpcID)
		}
		fmt.Printf("Created:   %s\n", cache.CreatedAt)
	},
}

var deleteCacheCmd = &cobra.Command{
	Use:   "rm [id]",
	Short: "Delete a cache instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		cacheID := resolveCacheID(args[0], client)
		if err := client.DeleteCache(cacheID); err != nil {
			fmt.Printf("Error deleting cache: %v\n", err)
			return
		}
		fmt.Println("Cache deleted successfully")
	},
}

var connectionCacheCmd = &cobra.Command{
	Use:   "connection [id]",
	Short: "Get connection string",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		cacheID := resolveCacheID(args[0], client)
		connStr, err := client.GetCacheConnectionString(cacheID)
		if err != nil {
			fmt.Printf("Error getting connection string: %v\n", err)
			return
		}
		fmt.Println(connStr)
	},
}

var statsCacheCmd = &cobra.Command{
	Use:   "stats [id]",
	Short: "Get cache statistics",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		cacheID := resolveCacheID(args[0], client)
		stats, err := client.GetCacheStats(cacheID)
		if err != nil {
			fmt.Printf("Error getting stats: %v\n", err)
			return
		}

		fmt.Printf("Used Memory: %s\n", formatBytes(stats.UsedMemoryBytes))
		fmt.Printf("Max Memory:  %s\n", formatBytes(stats.MaxMemoryBytes))
		fmt.Printf("Clients:     %d\n", stats.ConnectedClients)
		fmt.Printf("Keys:        %d\n", stats.TotalKeys)
	},
}

var flushCacheCmd = &cobra.Command{
	Use:   "flush [id]",
	Short: "Flush all keys from cache",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("yes")
		if !force {
			fmt.Println("WARNING: This will delete ALL keys in the cache. Use --yes to confirm.")
			return
		}

		client := createClient(opts)
		cacheID := resolveCacheID(args[0], client)
		if err := client.FlushCache(cacheID); err != nil {
			fmt.Printf("Error flushing cache: %v\n", err)
			return
		}
		fmt.Println("Cache flushed successfully")
	},
}

var resizeCacheCmd = &cobra.Command{
	Use:   "resize [id] [memoryMB]",
	Short: "Resize cache memory allocation",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		cacheID := resolveCacheID(args[0], client)
		memory, _ := strconv.Atoi(args[1])
		if err := client.ResizeCache(cacheID, memory); err != nil {
			fmt.Printf("Error resizing cache: %v\n", err)
			return
		}
		fmt.Printf("[SUCCESS] Cache %s resized to %d MB.\n", cacheID, memory)
	},
}

// resolveCacheID resolves a cache ID or name to a full UUID.
func resolveCacheID(idOrName string, client *sdk.Client) string {
	if _, err := uuid.Parse(idOrName); err == nil {
		return idOrName // Already a valid UUID
	}
	caches, err := client.ListCaches()
	if err != nil {
		return idOrName // Fallback
	}
	for _, c := range caches {
		if c.Name == idOrName {
			return c.ID
		}
	}
	return idOrName
}

func init() {
	createCacheCmd.Flags().String("name", "", "Name of the cache")
	createCacheCmd.Flags().String("version", "7.2", "Redis version")
	createCacheCmd.Flags().Int("memory", 128, "Memory limit in MB")
	createCacheCmd.Flags().String("vpc", "", "VPC ID to attach to")
	createCacheCmd.Flags().Bool("wait", false, "Wait for cache to be ready")
	_ = createCacheCmd.MarkFlagRequired("name")

	flushCacheCmd.Flags().Bool("yes", false, "Confirm flush")

	cacheCmd.AddCommand(createCacheCmd)
	cacheCmd.AddCommand(listCacheCmd)
	cacheCmd.AddCommand(getCacheCmd)
	cacheCmd.AddCommand(deleteCacheCmd)
	cacheCmd.AddCommand(connectionCacheCmd)
	cacheCmd.AddCommand(statsCacheCmd)
	cacheCmd.AddCommand(flushCacheCmd)
	cacheCmd.AddCommand(resizeCacheCmd)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
