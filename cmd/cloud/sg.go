package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

var sgCmd = &cobra.Command{
	Use:   "sg",
	Short: "Manage security groups",
}

var sgCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new security group",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vpcID, _ := cmd.Flags().GetString("vpc-id")
		desc, _ := cmd.Flags().GetString("description")

		if vpcID == "" {
			fmt.Println("Error: --vpc-id is required")
			return
		}

		client := getClient()
		sg, err := client.CreateSecurityGroup(vpcID, args[0], desc)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Security Group %s created successfully!\n", sg.Name)
		fmt.Printf("ID: %s\n", sg.ID)
		fmt.Printf("ARN: %s\n", sg.ARN)
	},
}

var sgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List security groups",
	Run: func(cmd *cobra.Command, args []string) {
		vpcID, _ := cmd.Flags().GetString("vpc-id")
		if vpcID == "" {
			fmt.Println("Error: --vpc-id is required")
			return
		}

		client := getClient()
		groups, err := client.ListSecurityGroups(vpcID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if outputJSON {
			data, _ := json.MarshalIndent(groups, "", "  ")
			fmt.Println(string(data))
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "VPC ID", "ARN"})

		for _, g := range groups {
			table.Append([]string{
				g.ID[:8],
				g.Name,
				g.VPCID[:8],
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

		client := getClient()
		rule := sdk.SecurityRule{
			Direction: direction,
			Protocol:  protocol,
			PortMin:   portMin,
			PortMax:   portMax,
			CIDR:      cidr,
			Priority:  priority,
		}

		res, err := client.AddSecurityRule(args[0], rule)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Rule %s added successfully to security group %s\n", res.ID, args[0])
	},
}

var sgAttachCmd = &cobra.Command{
	Use:   "attach [instance-id] [sg-id]",
	Short: "Attach a security group to an instance",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient()
		if err := client.AttachSecurityGroup(args[0], args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("[SUCCESS] Security Group %s attached to instance %s successfully.\n", args[1], args[0])
	},
}

func init() {
	sgCreateCmd.Flags().String("vpc-id", "", "VPC ID")
	sgCreateCmd.Flags().String("description", "", "Description")

	sgListCmd.Flags().String("vpc-id", "", "VPC ID")

	sgAddRuleCmd.Flags().String("direction", "ingress", "Rule direction (ingress/egress)")
	sgAddRuleCmd.Flags().String("protocol", "tcp", "Protocol (tcp/udp/icmp/all)")
	sgAddRuleCmd.Flags().Int("port-min", 0, "Minimum port")
	sgAddRuleCmd.Flags().Int("port-max", 0, "Maximum port")
	sgAddRuleCmd.Flags().String("cidr", "0.0.0.0/0", "CIDR block")
	sgAddRuleCmd.Flags().Int("priority", 100, "Priority")

	sgCmd.AddCommand(sgCreateCmd, sgListCmd, sgAddRuleCmd, sgAttachCmd)
	rootCmd.AddCommand(sgCmd)
}
