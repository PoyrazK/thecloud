// Package main provides the cloud CLI commands.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

var kubernetesCmd = &cobra.Command{
	Use:     "kubernetes",
	Aliases: []string{"k8s", "cluster"},
	Short:   "Manage Kubernetes clusters",
}

var listClustersCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Kubernetes clusters",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		clusters, err := client.ListClusters()
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(clusters, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "VERSION", "WORKERS", "STATUS"})

		for _, c := range clusters {
			id := c.ID.String()
			if len(id) > 8 {
				id = truncateID(id)
			}
			_ = table.Append([]string{
				id,
				c.Name,
				c.Version,
				fmt.Sprintf("%d", c.WorkerCount),
				c.Status,
			})
		}
		_ = table.Render()
	},
}

var createClusterCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Kubernetes cluster",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		vpcStr, _ := cmd.Flags().GetString("vpc")
		version, _ := cmd.Flags().GetString("version")
		workers, _ := cmd.Flags().GetInt("workers")

		vpcID, err := uuid.Parse(vpcStr)
		if err != nil {
			// If not UUID, maybe try to lookup by name?
			// For simplicity in MVP, we require UUID for VPCs in CLI here.
			// But we could implement a lookup if needed.
			fmt.Printf("Error: invalid VPC ID: %v\n", err)
			return
		}

		isolate, _ := cmd.Flags().GetBool("isolate")

		ha, _ := cmd.Flags().GetBool("ha")

		client := createClient(opts)
		cluster, err := client.CreateCluster(&sdk.CreateClusterInput{
			Name:             name,
			VpcID:            vpcID,
			Version:          version,
			WorkerCount:      workers,
			NetworkIsolation: isolate,
			HA:               ha,
		})
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		fmt.Println("[SUCCESS] Cluster creation initiated!")
		data, _ := json.MarshalIndent(cluster, "", "  ")
		fmt.Println(string(data))
	},
}

var deleteClusterCmd = &cobra.Command{
	Use:     "rm [id]",
	Aliases: []string{"delete"},
	Short:   "Delete a Kubernetes cluster",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.DeleteCluster(id); err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		fmt.Println("[INFO] Cluster deletion initiated.")
	},
}

var getKubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig [id]",
	Short: "Get kubeconfig for a cluster",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		role, _ := cmd.Flags().GetString("role")
		client := createClient(opts)
		kubeconfig, err := client.GetKubeconfig(id, role)
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		fmt.Print(kubeconfig)
	},
}

var repairClusterCmd = &cobra.Command{
	Use:   "repair [id]",
	Short: "Repair cluster components (CNI, kube-proxy)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.RepairCluster(id); err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}
		fmt.Println("[SUCCESS] Cluster repair initiated.")
	},
}

var scaleClusterCmd = &cobra.Command{
	Use:   "scale [id]",
	Short: "Scale cluster worker nodes",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		workers, _ := cmd.Flags().GetInt("workers")
		client := createClient(opts)
		if err := client.ScaleCluster(id, workers); err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}
		fmt.Printf("[SUCCESS] Scaling to %d workers initiated.\n", workers)
	},
}

var clusterHealthCmd = &cobra.Command{
	Use:   "health [id]",
	Short: "Check cluster operational health",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		health, err := client.GetClusterHealth(id)
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		fmt.Printf("\nCluster Health: %s\n", health.Status)
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf(fmtDetailRow, "API Server:", health.APIServer)
		fmt.Printf(fmtDetailRow, "Nodes Ready:", fmt.Sprintf("%d/%d", health.NodesReady, health.NodesTotal))
		if health.Message != "" {
			fmt.Printf(fmtDetailRow, "Message:", health.Message)
		}
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println("")
	},
}
var showClusterCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show detailed cluster information",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		cluster, err := client.GetCluster(id)
		if err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}

		fmt.Print("\nCluster Details\n")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf(fmtDetailRow, "ID:", cluster.ID)
		fmt.Printf(fmtDetailRow, "Name:", cluster.Name)
		fmt.Printf(fmtDetailRow, "Status:", cluster.Status)
		fmt.Printf(fmtDetailRow, "Version:", cluster.Version)
		fmt.Printf(fmtDetailRow, "VPC ID:", cluster.VpcID)
		fmt.Printf(fmtDetailRow, "Workers:", cluster.WorkerCount)
		fmt.Printf(fmtDetailRow, "HA Enabled:", cluster.HAEnabled)
		if cluster.APIServerLBAddress != nil {
			fmt.Printf(fmtDetailRow, "API Server LB:", *cluster.APIServerLBAddress)
		}
		fmt.Printf(fmtDetailRow, "Control Plane:", strings.Join(cluster.ControlPlaneIPs, ", "))
		fmt.Printf(fmtDetailRow, "Created At:", cluster.CreatedAt)
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println("")
	},
}

var upgradeClusterCmd = &cobra.Command{
	Use:   "upgrade [id]",
	Short: "Upgrade cluster Kubernetes version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		version, _ := cmd.Flags().GetString("version")
		client := createClient(opts)
		if err := client.UpgradeCluster(id, version); err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}
		fmt.Printf("[SUCCESS] Upgrade to %s initiated.\n", version)
	},
}

var rotateSecretsCmd = &cobra.Command{
	Use:   "rotate-secrets [id]",
	Short: "Rotate cluster certificates and secrets",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.RotateSecrets(id); err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}
		fmt.Println("[SUCCESS] Secret rotation completed.")
	},
}

var backupClusterCmd = &cobra.Command{
	Use:   "backup [id]",
	Short: "Create a point-in-time backup of cluster state",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.CreateBackup(id); err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}
		fmt.Println("[SUCCESS] Cluster backup initiated.")
	},
}

var restoreClusterCmd = &cobra.Command{
	Use:   "restore [id]",
	Short: "Restore cluster state from backup",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		path, _ := cmd.Flags().GetString("path")
		client := createClient(opts)
		if err := client.RestoreBackup(id, path); err != nil {
			fmt.Printf(fmtErrorLog, err)
			return
		}
		fmt.Println("[SUCCESS] Cluster restoration initiated.")
	},
}

func init() {
	kubernetesCmd.AddCommand(listClustersCmd)
	kubernetesCmd.AddCommand(createClusterCmd)
	kubernetesCmd.AddCommand(deleteClusterCmd)
	kubernetesCmd.AddCommand(getKubeconfigCmd)
	kubernetesCmd.AddCommand(showClusterCmd)
	kubernetesCmd.AddCommand(repairClusterCmd)
	kubernetesCmd.AddCommand(scaleClusterCmd)
	kubernetesCmd.AddCommand(clusterHealthCmd)
	kubernetesCmd.AddCommand(upgradeClusterCmd)
	kubernetesCmd.AddCommand(rotateSecretsCmd)
	kubernetesCmd.AddCommand(backupClusterCmd)

	kubernetesCmd.AddCommand(restoreClusterCmd)

	createClusterCmd.Flags().StringP("name", "n", "", "Name of the cluster (required)")
	createClusterCmd.Flags().StringP("vpc", "v", "", "VPC ID to use (required)")
	createClusterCmd.Flags().StringP("version", "V", "v1.29.0", "Kubernetes version")
	createClusterCmd.Flags().IntP("workers", "w", 2, "Number of worker nodes")
	createClusterCmd.Flags().Bool("isolate", false, "Enable strict network isolation (NetworkPolicies)")
	createClusterCmd.Flags().Bool("ha", false, "Enable High-Availability control plane (3 masters + LB)")
	_ = createClusterCmd.MarkFlagRequired("name")
	_ = createClusterCmd.MarkFlagRequired("vpc")

	getKubeconfigCmd.Flags().StringP("role", "r", "admin", "Role for kubeconfig (admin, viewer)")
	scaleClusterCmd.Flags().IntP("workers", "w", 2, "Target number of worker nodes")
	_ = scaleClusterCmd.MarkFlagRequired("workers")

	upgradeClusterCmd.Flags().StringP("version", "V", "", "Target Kubernetes version (required)")
	_ = upgradeClusterCmd.MarkFlagRequired("version")

	restoreClusterCmd.Flags().StringP("path", "p", "", "Path to backup file (required)")
	_ = restoreClusterCmd.MarkFlagRequired("path")
}
