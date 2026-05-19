// Package main provides the cloud CLI entrypoint.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

const autoscalingErrorFormat = "Error: %v\n"

var autoscalingCmd = &cobra.Command{
	Use:     "autoscaling",
	Aliases: []string{"asg"},
	Short:   "Manage auto-scaling groups",
}

var asgCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new scaling group",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		vpcID, _ := cmd.Flags().GetString("vpc")
		image, _ := cmd.Flags().GetString("image")
		min, _ := cmd.Flags().GetInt("min")
		max, _ := cmd.Flags().GetInt("max")
		desired, _ := cmd.Flags().GetInt("desired")
		lbID, _ := cmd.Flags().GetString("lb")
		ports, _ := cmd.Flags().GetString("ports")

		client := createClient(opts)

		req := sdk.CreateScalingGroupRequest{
			Name:         name,
			VpcID:        vpcID,
			Image:        image,
			MinInstances: min,
			MaxInstances: max,
			DesiredCount: desired,
			Ports:        ports,
		}
		if lbID != "" {
			req.LoadBalancerID = &lbID
		}

		group, err := client.CreateScalingGroup(req)
		if err != nil {
			fmt.Printf(autoscalingErrorFormat, err)
			os.Exit(1)
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(group, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Printf("[SUCCESS] Scaling Group %s created (ID: %s)\n", group.Name, group.ID)
	},
}

var asgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scaling groups",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		groups, err := client.ListScalingGroups()
		if err != nil {
			fmt.Printf(autoscalingErrorFormat, err)
			os.Exit(1)
		}

		if opts.JSON {
			data, _ := json.MarshalIndent(groups, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "INSTANCES (Cur/Des/Min/Max)", "STATUS"})

		for _, g := range groups {
			instances := fmt.Sprintf("%d / %d / %d / %d", g.CurrentCount, g.DesiredCount, g.MinInstances, g.MaxInstances)
			_ = table.Append([]string{g.ID, g.Name, instances, g.Status})
		}
		_ = table.Render()
	},
}

var asgRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Delete a scaling group",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		if err := client.DeleteScalingGroup(args[0]); err != nil {
			fmt.Printf(autoscalingErrorFormat, err)
			os.Exit(1)
		}
		fmt.Println("[SUCCESS] Scaling Group deleted")
	},
}

var asgPolicyAddCmd = &cobra.Command{
	Use:   "add-policy <group-id>",
	Short: "Add a scaling policy",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		metric, _ := cmd.Flags().GetString("metric")
		target, _ := cmd.Flags().GetFloat64("target")
		scaleOut, _ := cmd.Flags().GetInt("scale-out")
		scaleIn, _ := cmd.Flags().GetInt("scale-in")
		cooldown, _ := cmd.Flags().GetInt("cooldown")

		client := createClient(opts)
		req := sdk.CreatePolicyRequest{
			Name:        name,
			MetricType:  metric,
			TargetValue: target,
			ScaleOut:    scaleOut,
			ScaleIn:     scaleIn,
			CooldownSec: cooldown,
		}

		if err := client.CreateScalingPolicy(args[0], req); err != nil {
			fmt.Printf(autoscalingErrorFormat, err)
			os.Exit(1)
		}
		fmt.Println("[SUCCESS] Policy added")
	},
}

func init() {
	asgCreateCmd.Flags().StringP("name", "n", "", "Group Name")
	asgCreateCmd.Flags().StringP("vpc", "v", "", "VPC ID")
	asgCreateCmd.Flags().String("image", "", "Docker Image")
	asgCreateCmd.Flags().String("lb", "", "Load Balancer ID (Optional)")
	asgCreateCmd.Flags().String("ports", "", "Ports (e.g. 8080:80)")
	asgCreateCmd.Flags().Int("min", 1, "Min instances")
	asgCreateCmd.Flags().Int("max", 5, "Max instances")
	asgCreateCmd.Flags().Int("desired", 1, "Desired instances")
	cobra.CheckErr(asgCreateCmd.MarkFlagRequired("name"))
	cobra.CheckErr(asgCreateCmd.MarkFlagRequired("vpc"))
	cobra.CheckErr(asgCreateCmd.MarkFlagRequired("image"))

	asgPolicyAddCmd.Flags().StringP("name", "n", "", "Policy Name")
	asgPolicyAddCmd.Flags().String("metric", "cpu", "Metric Type (cpu|memory)")
	asgPolicyAddCmd.Flags().Float64("target", 80.0, "Target Value")
	asgPolicyAddCmd.Flags().Int("scale-out", 1, "Scale out step")
	asgPolicyAddCmd.Flags().Int("scale-in", 1, "Scale in step")
	asgPolicyAddCmd.Flags().Int("cooldown", 300, "Cooldown seconds")
	cobra.CheckErr(asgPolicyAddCmd.MarkFlagRequired("name"))

	autoscalingCmd.AddCommand(asgCreateCmd)
	autoscalingCmd.AddCommand(asgListCmd)
	autoscalingCmd.AddCommand(asgRmCmd)
	autoscalingCmd.AddCommand(asgPolicyAddCmd)
}
