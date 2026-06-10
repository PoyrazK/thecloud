// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

var sgCmd = &cobra.Command{
	Use:   "sg",
	Short: "Manage security groups",
}

const (
	flagVPCID      = "vpc-id"
	descVPCID      = "VPC ID"
	msgRuleRemoved = "[SUCCESS] Rule %s removed successfully.\n"
	msgSgDetached  = "[SUCCESS] Security Group %s detached from instance %s successfully.\n"
)

var sgCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new security group",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vpcID, _ := cmd.Flags().GetString(flagVPCID)
		desc, _ := cmd.Flags().GetString("description")

		if vpcID == "" {
			fmt.Printf("Error: --%s is required\n", flagVPCID)
			return
		}

		client := createClient(opts)
		sg, err := client.CreateSecurityGroup(vpcID, args[0], desc)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			printJSON(sg)
		} else {
			fmt.Printf("[SUCCESS] Security Group %s created successfully!\n", sg.Name)
			fmt.Printf("ID: %s\n", sg.ID)
			fmt.Printf("ARN: %s\n", sg.ARN)
		}
	},
}

var sgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List security groups",
	Run: func(cmd *cobra.Command, args []string) {
		vpcID, _ := cmd.Flags().GetString(flagVPCID)

		client := createClient(opts)
		groups, err := client.ListSecurityGroups(vpcID)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			printJSON(groups)
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", descVPCID, "ARN"})

		for _, g := range groups {
			table.Append([]string{
				truncateID(g.ID),
				g.Name,
				truncateID(g.VPCID),
				g.ARN,
			})
		}
		table.Render()
	},
}

var sgAddRuleCmd = &cobra.Command{
	Use:   "add-rule [sg-id]",
	Short: "Add a rule to a security group",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		direction, _ := cmd.Flags().GetString("direction")
		protocol, _ := cmd.Flags().GetString("protocol")
		portMin, _ := cmd.Flags().GetInt("port-min")
		portMax, _ := cmd.Flags().GetInt("port-max")
		cidr, _ := cmd.Flags().GetString("cidr")
		priority, _ := cmd.Flags().GetInt("priority")

		client := createClient(opts)
		sgID := resolveSGID(args[0], client)
		rule := sdk.SecurityRule{
			Direction: direction,
			Protocol:  protocol,
			PortMin:   portMin,
			PortMax:   portMax,
			CIDR:      cidr,
			Priority:  priority,
		}

		res, err := client.AddSecurityRule(sgID, rule)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		fmt.Printf("[SUCCESS] Rule %s added successfully to security group %s\n", res.ID, sgID)
	},
}

var sgAttachCmd = &cobra.Command{
	Use:   "attach [instance-id] [sg-id]",
	Short: "Attach a security group to an instance",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		if err := client.AttachSecurityGroup(args[0], args[1]); err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		fmt.Printf("[SUCCESS] Security Group %s attached to instance %s successfully.\n", args[1], args[0])
	},
}

var sgRemoveRuleCmd = &cobra.Command{
	Use:   "remove-rule [rule-id]",
	Short: "Remove a rule from a security group",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		if err := client.RemoveSecurityRule(args[0]); err != nil {
			fmt.Printf(errFmt, err)
			return
		}
		fmt.Printf(msgRuleRemoved, args[0])
	},
}

var sgDetachCmd = &cobra.Command{
	Use:   "detach [instance-id] [sg-id]",
	Short: "Detach a security group from an instance",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		if err := client.DetachSecurityGroup(args[0], args[1]); err != nil {
			fmt.Printf(errFmt, err)
			return
		}
		fmt.Printf(msgSgDetached, args[1], args[0])
	},
}

var sgGetCmd = &cobra.Command{
	Use:   "get [sg-id]",
	Short: "Get security group details and rules",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		sgID := resolveSGID(args[0], client)
		sg, err := client.GetSecurityGroup(sgID)
		if err != nil {
			fmt.Printf(errFmt, err)
			return
		}

		if opts.JSON {
			printJSON(sg)
			return
		}

		fmt.Printf("Security Group: %s (%s)\n", sg.Name, sg.ID)
		fmt.Printf("VPC: %s\n", sg.VPCID)
		fmt.Printf("Description: %s\n", sg.Description)
		fmt.Printf("ARN: %s\n", sg.ARN)
		fmt.Println("\nRules:")

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"Rule ID", "Direction", "Protocol", "Ports", "CIDR", "Priority"})

		for _, r := range sg.Rules {
			ports := fmt.Sprintf("%d-%d", r.PortMin, r.PortMax)
			if r.PortMin == r.PortMax {
				ports = fmt.Sprintf("%d", r.PortMin)
			}
			table.Append([]string{
				truncateID(r.ID),
				r.Direction,
				r.Protocol,
				ports,
				r.CIDR,
				fmt.Sprintf("%d", r.Priority),
			})
		}
		table.Render()
	},
}

var sgDeleteCmd = &cobra.Command{
	Use:     "rm [sg-id]",
	Aliases: []string{"delete"},
	Short:   "Delete a security group",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := createClient(opts)
		sgID := resolveSGID(args[0], client)
		if err := client.DeleteSecurityGroup(sgID); err != nil {
			fmt.Printf(errFmt, err)
			return
		}
		fmt.Printf("[SUCCESS] Security Group %s deleted successfully.\n", args[0])
	},
}

func init() {
	sgCreateCmd.Flags().String(flagVPCID, "", descVPCID)
	sgCreateCmd.Flags().String("description", "", "Description")

	sgListCmd.Flags().String(flagVPCID, "", descVPCID)

	sgAddRuleCmd.Flags().String("direction", "ingress", "Rule direction (ingress/egress)")
	sgAddRuleCmd.Flags().String("protocol", "tcp", "Protocol (tcp/udp/icmp/all)")
	sgAddRuleCmd.Flags().Int("port-min", 0, "Minimum port")
	sgAddRuleCmd.Flags().Int("port-max", 0, "Maximum port")
	sgAddRuleCmd.Flags().String("cidr", "0.0.0.0/0", "CIDR block")
	sgAddRuleCmd.Flags().Int("priority", 100, "Priority")

	sgCmd.AddCommand(sgCreateCmd, sgListCmd, sgGetCmd, sgDeleteCmd, sgAddRuleCmd, sgRemoveRuleCmd, sgAttachCmd, sgDetachCmd)
}

// resolveSGID resolves a security group ID or name to a full UUID.
func resolveSGID(idOrName string, client *sdk.Client) string {
	if _, err := uuid.Parse(idOrName); err == nil {
		return idOrName
	}
	// Try to find by name - list all VPCs and check each for the security group
	vpcs, err := client.ListVPCs()
	if err != nil {
		return idOrName
	}
	for _, vpc := range vpcs {
		groups, err := client.ListSecurityGroups(vpc.ID)
		if err != nil {
			continue
		}
		for _, g := range groups {
			if g.Name == idOrName {
				return g.ID
			}
		}
	}
	return idOrName
}
