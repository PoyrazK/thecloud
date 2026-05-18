// Package main provides the cloud CLI entrypoint.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"strings"

	"os/exec"
	"syscall"

	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

const (
	fmtErrorLog     = "Error: %v\n"
	fmtDetailRow    = "%-15s %v\n"
	successInstance = "[SUCCESS] Instance launched successfully!\n"
	infoStop        = "[INFO] Instance stop initiated."
	pollInterval    = 2 * time.Second
)

var instanceCmd = &cobra.Command{
	Use:   "instance",
	Short: "Manage compute instances",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all instances",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)

		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")

		instances, meta, err := listWithPagination(
			client.ListInstances,
			client.ListInstancesWithPagination,
			limit, offset,
		)
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		if opts.JSON {
			if meta != nil {
				printJSON(meta)
			} else {
				printJSON(instances)
			}
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "IMAGE", "STATUS", "ACCESS"})

		for _, inst := range instances {
			id := truncateID(inst.ID)

			access := formatAccessPorts(inst.Ports, inst.Status)

			table.Append([]string{
				id,
				inst.Name,
				inst.Image,
				inst.Status,
				access,
			})
		}
		table.Render()

		if meta != nil {
			fmt.Printf("\nShowing %d of %d total", len(instances), meta.TotalCount)
			if meta.HasMore {
				fmt.Print(" (more available)")
			}
			fmt.Println()
		}
	},
}

var launchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch a new instance",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		image, _ := cmd.Flags().GetString("image")
		ports, _ := cmd.Flags().GetString("port")
		instanceType, _ := cmd.Flags().GetString("type")
		vpc, _ := cmd.Flags().GetString("vpc")
		subnetID, _ := cmd.Flags().GetString("subnet")
		volumeStrs, _ := cmd.Flags().GetStringSlice("volume")
		sshKeyID, _ := cmd.Flags().GetString("ssh-key")
		wait, _ := cmd.Flags().GetBool("wait")

		// Parse volume strings like "vol-name:/path"
		var volumes []sdk.VolumeAttachmentInput
		for _, v := range volumeStrs {
			parts := strings.SplitN(v, ":", 2)
			if len(parts) == 2 {
				volumes = append(volumes, sdk.VolumeAttachmentInput{
					VolumeID:  parts[0],
					MountPath: parts[1],
				})
			}
		}
		metaStrs, _ := cmd.Flags().GetStringSlice("metadata")
		metadata := make(map[string]string)
		for _, s := range metaStrs {
			parts := strings.SplitN(s, "=", 2)
			if len(parts) == 2 {
				metadata[parts[0]] = parts[1]
			}
		}

		labelStrs, _ := cmd.Flags().GetStringSlice("label")
		labels := make(map[string]string)
		for _, s := range labelStrs {
			parts := strings.SplitN(s, "=", 2)
			if len(parts) == 2 {
				labels[parts[0]] = parts[1]
			}
		}

		runCmd, _ := cmd.Flags().GetStringSlice("cmd")
		client := createClient(opts)
		inst, err := client.LaunchInstance(name, image, ports, instanceType, vpc, subnetID, volumes, metadata, labels, sshKeyID, runCmd)
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		fmt.Print(successInstance)

		if wait {
			shortID := truncateID(inst.ID)
			fmt.Printf("[INFO] Waiting for instance %s to be RUNNING...\n", shortID)
			for {
				updated, err := client.GetInstance(inst.ID)
				if err != nil {
					fmt.Printf("\nError polling status: %v\n", err)
					return
				}
				if updated.Status == "RUNNING" {
					inst = updated
					fmt.Println("\n[SUCCESS] Instance is now RUNNING.")
					break
				}
				if updated.Status == "FAILED" || updated.Status == "TERMINATED" {
					fmt.Printf("\n[ERROR] Instance entered state: %s\n", updated.Status)
					return
				}
				fmt.Print(".")
				time.Sleep(pollInterval)
			}
		}

		if opts.JSON {
			printJSON(inst)
		} else {
			data, _ := json.MarshalIndent(inst, "", "  ")
			fmt.Println(string(data))
		}
	},
}
var stopCmd = &cobra.Command{
	Use:   "stop [id]",
	Short: "Stop an instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.StopInstance(id); err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		fmt.Println(infoStop)
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs [id]",
	Short: "View instance logs",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		logs, err := client.GetInstanceLogs(id)
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		fmt.Print(logs)
	},
}

var showCmd = &cobra.Command{
	Use:   "show [id/name]",
	Short: "Show detailed instance information",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		inst, err := client.GetInstance(id)
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		if opts.JSON {
			printJSON(inst)
			return
		}

		fmt.Print("\nInstance Details\n")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf(fmtDetailRow, "ID:", inst.ID)
		fmt.Printf(fmtDetailRow, "Name:", inst.Name)
		fmt.Printf(fmtDetailRow, "Status:", inst.Status)
		fmt.Printf(fmtDetailRow, "Image:", inst.Image)
		fmt.Printf(fmtDetailRow, "Ports:", inst.Ports)
		fmt.Printf(fmtDetailRow, "Created At:", inst.CreatedAt)
		fmt.Printf(fmtDetailRow, "Version:", inst.Version)
		fmt.Printf(fmtDetailRow, "Container ID:", inst.ContainerID)
		if len(inst.Metadata) > 0 {
			metaStr, _ := json.Marshal(inst.Metadata)
			fmt.Printf(fmtDetailRow, "Metadata:", string(metaStr))
		}
		if len(inst.Labels) > 0 {
			labelStr, _ := json.Marshal(inst.Labels)
			fmt.Printf(fmtDetailRow, "Labels:", string(labelStr))
		}
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println("")
	},
}

var consoleCmd = &cobra.Command{
	Use:   "console [id/name]",
	Short: "Get the VNC console URL for an instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		url, err := client.GetConsoleURL(id)
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		fmt.Printf("VNC Console URL: %s\n", url)
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm [id/name]",
	Short: "Remove an instance and its resources",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.TerminateInstance(id); err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

	},
}

var resizeCmd = &cobra.Command{
	Use:   "resize [id/name] [instance-type]",
	Short: "Resize an instance to a new instance type",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		instanceType := args[1]
		client := createClient(opts)
		if err := client.ResizeInstance(id, instanceType); err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}
		fmt.Printf("[SUCCESS] Instance %s resized to %s.\n", id, instanceType)
	},
}

var statsCmd = &cobra.Command{
	Use:   "stats [id/name]",
	Short: "Show instance statistics (CPU/Mem)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		stats, err := client.GetInstanceStats(id)
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		if opts.JSON {
			printJSON(stats)
			return
		}

		fmt.Printf("\nStatistics for %s\n", id)
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("CPU:    %.2f%%\n", stats.CPUPercentage)
		fmt.Printf("Memory: %.2f%% (%.2f MB / %.2f MB)\n",
			stats.MemoryPercentage,
			stats.MemoryUsageBytes/1024/1024,
			stats.MemoryLimitBytes/1024/1024)
	},
}

var metadataCmd = &cobra.Command{
	Use:   "metadata [id/name]",
	Short: "Update instance metadata or labels",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		metaStrs, _ := cmd.Flags().GetStringSlice("metadata")
		labelStrs, _ := cmd.Flags().GetStringSlice("label")

		metadata := make(map[string]string)
		for _, s := range metaStrs {
			parts := strings.SplitN(s, "=", 2)
			if len(parts) == 2 {
				metadata[parts[0]] = parts[1]
			}
		}

		labels := make(map[string]string)
		for _, s := range labelStrs {
			parts := strings.SplitN(s, "=", 2)
			if len(parts) == 2 {
				labels[parts[0]] = parts[1]
			}
		}

		client := createClient(opts)
		if err := client.UpdateInstanceMetadata(id, metadata, labels); err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}
		fmt.Println("[SUCCESS] Metadata updated.")
	},
}

var sshCmd = &cobra.Command{
	Use:   "ssh [id/name]",
	Short: "SSH into an instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		keyPath, _ := cmd.Flags().GetString("i")
		user, _ := cmd.Flags().GetString("user")

		client := createClient(opts)
		inst, err := client.GetInstance(id)
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		if inst.Status != "RUNNING" {
			fmt.Printf("Error: Instance is in %s state, must be RUNNING to SSH\n", inst.Status)
			return
		}

		// Find SSH port (22)
		var hostPort string
		pList := strings.Split(inst.Ports, ",")
		for _, mapping := range pList {
			parts := strings.Split(mapping, ":")
			if len(parts) == 2 && parts[1] == "22" {
				hostPort = parts[0]
				break
			}
		}

		var targetHost string
		var targetPort string

		if hostPort != "" {
			targetHost = "localhost"
			targetPort = hostPort
		} else if inst.PrivateIP != "" {
			targetHost = inst.PrivateIP
			targetPort = "22"
		} else {
			fmt.Println("Error: No SSH access available. Neither port mapping for 22 nor Private IP found.")
			return
		}

		sshArgs := []string{"-p", targetPort, "-o", "StrictHostKeyChecking=no"}
		if keyPath != "" {
			sshArgs = append(sshArgs, "-i", keyPath)
		}
		sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", user, targetHost))

		fmt.Printf("[INFO] Connecting to %s (%s:%s)...\n", inst.Name, targetHost, targetPort)

		binary, err := exec.LookPath("ssh")
		if err != nil {
			fmt.Printf("Error: ssh client not found in PATH: %v\n", err)
			return
		}

		// Use syscall.Exec to replace CLI process with ssh process
		// This handles interactive terminal correctly.
		env := os.Environ()
		allArgs := append([]string{"ssh"}, sshArgs...)
		err = syscall.Exec(binary, allArgs, env)
		if err != nil {
			fmt.Printf("Error executing ssh: %v\n", err)
		}
	},
}

func init() {
	instanceCmd.AddCommand(listCmd)
	instanceCmd.AddCommand(launchCmd)
	instanceCmd.AddCommand(stopCmd)
	instanceCmd.AddCommand(logsCmd)
	instanceCmd.AddCommand(showCmd)
	instanceCmd.AddCommand(consoleCmd)
	instanceCmd.AddCommand(rmCmd)
	instanceCmd.AddCommand(resizeCmd)
	instanceCmd.AddCommand(statsCmd)
	instanceCmd.AddCommand(metadataCmd)
	instanceCmd.AddCommand(sshCmd)

	launchCmd.Flags().StringP("name", "n", "", "Name of the instance (required)")
	launchCmd.Flags().StringP("image", "i", "alpine", "Image to use")
	launchCmd.Flags().StringP("port", "p", "", "Port mapping (host:container)")
	launchCmd.Flags().StringP("type", "t", "basic-2", "Instance type (e.g. basic-1, standard-1)")
	launchCmd.Flags().StringP("vpc", "v", "", "VPC ID or Name to attach to")
	launchCmd.Flags().StringP("subnet", "s", "", "Subnet ID or Name to attach to")
	launchCmd.Flags().StringSliceP("volume", "V", nil, "Volume attachment (vol-name:/path)")
	launchCmd.Flags().StringSliceP("metadata", "m", nil, "Metadata (key=value)")
	launchCmd.Flags().StringSliceP("label", "l", nil, "Labels (key=value)")
	launchCmd.Flags().String("ssh-key", "", "SSH Key ID to inject")
	launchCmd.Flags().StringSlice("cmd", nil, "Command to run (e.g. --cmd 'sh' --cmd '-c' --cmd 'echo hello')")
	launchCmd.Flags().BoolP("wait", "w", false, "Wait for instance to be RUNNING")
	_ = launchCmd.MarkFlagRequired("name")

	metadataCmd.Flags().StringSliceP("metadata", "m", nil, "Metadata (key=value)")
	metadataCmd.Flags().StringSliceP("label", "l", nil, "Labels (key=value)")

	listCmd.Flags().Int("limit", 0, "Maximum number of results (0 = use server default)")
	listCmd.Flags().Int("offset", 0, "Number of results to skip")

	sshCmd.Flags().StringP("i", "i", "", "Identity file (private key path)")
	sshCmd.Flags().StringP("user", "u", "root", "User to log in as")
}

func formatAccessPorts(ports string, status string) string {
	if ports == "" || status != "RUNNING" {
		return "-"
	}

	pList := strings.Split(ports, ",")
	var mappings []string
	for _, mapping := range pList {
		parts := strings.Split(mapping, ":")
		if len(parts) == 2 {
			mappings = append(mappings, fmt.Sprintf("localhost:%s->%s", parts[0], parts[1]))
		}
	}

	if len(mappings) == 0 {
		return ""
	}

	return strings.Join(mappings, ", ")
}
