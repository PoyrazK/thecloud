// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const loadBalancerErrorFormat = "Error: %v\n"

var lbCmd = &cobra.Command{
	Use:   "lb",
	Short: "Manage Load Balancers",
}

var lbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all load balancers",
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		lbs, err := client.ListLBs()
		if err != nil {
			fmt.Printf(loadBalancerErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(lbs)
			return
		}

		if len(lbs) == 0 {
			fmt.Println("No load balancers found.")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "VPC ID", "PORT", "ALGO", "STATUS"})

		for _, v := range lbs {
			table.Append([]string{
				truncateID(v.ID),
				v.Name,
				truncateID(v.VpcID),
				fmt.Sprintf("%d", v.Port),
				v.Algorithm,
				string(v.Status),
			})
		}
		table.Render()
	},
}

var lbCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new load balancer",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		vpcID, _ := cmd.Flags().GetString("vpc")
		port, _ := cmd.Flags().GetInt("port")
		algo, _ := cmd.Flags().GetString("algorithm")

		client := createClient(opts)
		lb, err := client.CreateLB(name, vpcID, port, algo)
		if err != nil {
			fmt.Printf(loadBalancerErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(lb)
		} else {
			fmt.Printf("[SUCCESS] Load Balancer %s creation initiated!\n", lb.Name)
			fmt.Printf("ID: %s\n", lb.ID)
			fmt.Printf("Status: %s (It will be ACTIVE shortly)\n", lb.Status)
		}
	},
}

var lbRmCmd = &cobra.Command{
	Use:     "rm [id]",
	Aliases: []string{"delete"},
	Short:   "Remove a load balancer",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := createClient(opts)
		if err := client.DeleteLB(id); err != nil {
			fmt.Printf(loadBalancerErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Load Balancer %s deletion initiated.\n", id)
	},
}

var lbAddTargetCmd = &cobra.Command{
	Use:   "add-target [lb-id]",
	Short: "Add a target instance to a load balancer",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		lbID := args[0]
		instID, _ := cmd.Flags().GetString("instance")
		port, _ := cmd.Flags().GetInt("port")
		weight, _ := cmd.Flags().GetInt("weight")

		client := createClient(opts)
		if err := client.AddLBTarget(lbID, instID, port, weight); err != nil {
			fmt.Printf(loadBalancerErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Target %s added to LB %s.\n", instID, lbID)
	},
}

var lbRemoveTargetCmd = &cobra.Command{
	Use:   "rm-target [lb-id] [instance-id]",
	Short: "Remove a target instance from a load balancer",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		lbID := args[0]
		instID := args[1]
		client := createClient(opts)
		if err := client.RemoveLBTarget(lbID, instID); err != nil {
			fmt.Printf(loadBalancerErrorFormat, err)
			return
		}

		fmt.Printf("[SUCCESS] Target %s removed from LB %s.\n", instID, lbID)
	},
}

func init() {
	lbCreateCmd.Flags().StringP("name", "n", "", "Name of the load balancer")
	cobra.CheckErr(lbCreateCmd.MarkFlagRequired("name"))
	lbCreateCmd.Flags().StringP("vpc", "v", "", "VPC ID")
	cobra.CheckErr(lbCreateCmd.MarkFlagRequired("vpc"))
	lbCreateCmd.Flags().Int("port", 80, "Public port for the LB")
	lbCreateCmd.Flags().String("algorithm", "round-robin", "LB algorithm (round-robin or least-conn)")

	lbAddTargetCmd.Flags().String("instance", "", "Target instance ID")
	cobra.CheckErr(lbAddTargetCmd.MarkFlagRequired("instance"))
	lbAddTargetCmd.Flags().Int("port", 80, "Port on the instance")
	cobra.CheckErr(lbAddTargetCmd.MarkFlagRequired("port"))
	lbAddTargetCmd.Flags().Int("weight", 1, "Weight for the target (optional)")

	lbCmd.AddCommand(lbListCmd)
	lbCmd.AddCommand(lbCreateCmd)
	lbCmd.AddCommand(lbRmCmd)
	lbCmd.AddCommand(lbAddTargetCmd)
	lbCmd.AddCommand(lbRemoveTargetCmd)
	lbCmd.AddCommand(lbListTargetsCmd)
}

var lbListTargetsCmd = &cobra.Command{
	Use:   "targets <lb-id>",
	Short: "List targets for a load balancer",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		targets, err := client.ListLBTargets(args[0])
		if err != nil {
			fmt.Printf(loadBalancerErrorFormat, err)
			return
		}

		if opts.JSON {
			printJSON(targets)
			return
		}

		if len(targets) == 0 {
			fmt.Println("No targets found for this load balancer.")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"INSTANCE ID", "PORT", "WEIGHT", "HEALTH"})
		for _, t := range targets {
			table.Append([]string{
				truncateID(t.InstanceID),
				fmt.Sprintf("%d", t.Port),
				fmt.Sprintf("%d", t.Weight),
				t.Health,
			})
		}
		table.Render()
	},
}
