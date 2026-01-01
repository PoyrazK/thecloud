package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/olekukonko/tablewriter"
	"github.com/poyraz/cloud/pkg/sdk"
)

var apiURL = "http://localhost:8080"
var apiKey string
var sdkClient *sdk.Client

func main() {
	// 1. Auth Setup
	apiKey = os.Getenv("MINIAWS_API_KEY")
	if apiKey == "" {
		fmt.Println("[WARN] MINIAWS_API_KEY not set.")
		createDemo := false
		prompt := &survey.Confirm{
			Message: "Would you like to generate a temporary key for this session?",
			Default: true,
		}
		survey.AskOne(prompt, &createDemo)

		if createDemo {
			var name string
			namePrompt := &survey.Input{
				Message: "Enter a name for your key (e.g. demo-user):",
				Default: "demo-user",
			}
			survey.AskOne(namePrompt, &name)

			tempClient := sdk.NewClient(apiURL, "")
			key, err := tempClient.CreateKey(name)
			if err == nil {
				apiKey = key
				fmt.Printf("[INFO] Generated Key: %s\n\n", apiKey)
			} else {
				fmt.Println("[ERROR] Failed to generate key. Falling back to manual input.")
			}
		}

		if apiKey == "" {
			manualPrompt := &survey.Input{
				Message: "Enter your API Key:",
			}
			survey.AskOne(manualPrompt, &apiKey)
		}
	}

	sdkClient = sdk.NewClient(apiURL, apiKey)

	for {
		mode := ""
		prompt := &survey.Select{
			Message: "Cloud CLI Control Panel - What would you like to do?",
			Options: []string{"List Instances", "Launch Instance", "Stop Instance", "Remove Instance", "View Logs", "View Details", "Manage VPCs", "Exit"},
		}
		if err := survey.AskOne(prompt, &mode); err != nil {
			fmt.Println("Bye!")
			return
		}

		switch mode {
		case "List Instances":
			listInstances()
		case "Launch Instance":
			launchInstance()
		case "Stop Instance":
			stopInstance()
		case "Remove Instance":
			removeInstance()
		case "View Logs":
			viewLogs()
		case "View Details":
			showInstance()
		case "Manage VPCs":
			vpcMenu()
		case "Exit":
			fmt.Println("See you in the cloud!")
			return
		}
		fmt.Println("")
	}
}

func listInstances() {
	instances, err := sdkClient.ListInstances()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Print("\033[H\033[2J") // Clear screen
	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"ID", "NAME", "IMAGE", "STATUS", "ACCESS"})

	for _, inst := range instances {
		access := "-"
		if inst.Ports != "" && inst.Status == "RUNNING" {
			pList := strings.Split(inst.Ports, ",")
			var mappings []string
			for _, mapping := range pList {
				parts := strings.Split(mapping, ":")
				if len(parts) == 2 {
					mappings = append(mappings, fmt.Sprintf("localhost:%s->%s", parts[0], parts[1]))
				}
			}
			access = strings.Join(mappings, ", ")
		}

		table.Append([]string{
			inst.ID[:8],
			inst.Name,
			inst.Image,
			inst.Status,
			access,
		})
	}
	table.Render()
}

func launchInstance() {
	qs := []*survey.Question{
		{
			Name:     "name",
			Prompt:   &survey.Input{Message: "Instance Name:"},
			Validate: survey.Required,
		},
		{
			Name: "image",
			Prompt: &survey.Select{
				Message: "Choose Image:",
				Options: []string{"alpine", "nginx:alpine", "ubuntu", "redis:alpine"},
				Default: "alpine",
			},
		},
		{
			Name: "ports",
			Prompt: &survey.Input{
				Message: "Port Mappings (host:container, optional):",
				Help:    "e.g. 8080:80",
			},
		},
	}

	answers := struct {
		Name  string
		Image string
		Ports string
	}{}

	if err := survey.Ask(qs, &answers); err != nil {
		return
	}

	// VPC Selection
	vpcs, _ := sdkClient.ListVPCs()
	vpcID := ""
	if len(vpcs) > 0 {
		var vpcOptions []string
		vpcOptions = append(vpcOptions, "None (Default Bridge)")
		for _, v := range vpcs {
			vpcOptions = append(vpcOptions, fmt.Sprintf("%s (%s)", v.Name, v.ID[:8]))
		}

		vpcChoice := ""
		prompt := &survey.Select{
			Message: "Attach to VPC?",
			Options: vpcOptions,
		}
		survey.AskOne(prompt, &vpcChoice)

		if vpcChoice != "None (Default Bridge)" {
			// Extract ID
			for _, v := range vpcs {
				if vpcChoice == fmt.Sprintf("%s (%s)", v.Name, v.ID[:8]) {
					vpcID = v.ID
					break
				}
			}
		}
	}

	inst, err := sdkClient.LaunchInstance(answers.Name, answers.Image, answers.Ports, vpcID)
	if err != nil {
		fmt.Printf("[ERROR] Failed: %v\n", err)
		return
	}

	fmt.Printf("[SUCCESS] Launched %s (%s) successfully!\n", inst.Name, inst.Image)
}

func selectInstance(message string, statusFilter string) *sdk.Instance {
	instances, err := sdkClient.ListInstances()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}

	var options []string
	instMap := make(map[string]sdk.Instance)

	for _, inst := range instances {
		if statusFilter != "" && inst.Status != statusFilter {
			continue
		}
		display := fmt.Sprintf("%s (%s) [%s]", inst.Name, inst.ID[:8], inst.Status)
		options = append(options, display)
		instMap[display] = inst
	}

	if len(instances) == 0 {
		fmt.Println("[WARN] No matching instances found.")
		return nil
	}

	var selected string
	prompt := &survey.Select{
		Message: message,
		Options: options,
	}
	if err := survey.AskOne(prompt, &selected); err != nil {
		return nil
	}

	inst := instMap[selected]
	return &inst
}

func stopInstance() {
	inst := selectInstance("Select instance to stop:", "RUNNING")
	if inst == nil {
		return
	}

	if err := sdkClient.StopInstance(inst.ID); err != nil {
		fmt.Printf("[ERROR] Failed to stop: %v\n", err)
		return
	}

	fmt.Printf("[INFO] Stopping %s...\n", inst.Name)
}

func removeInstance() {
	inst := selectInstance("Select instance to REMOVE (permanent):", "")
	if inst == nil {
		return
	}

	if err := sdkClient.TerminateInstance(inst.ID); err != nil {
		fmt.Printf("[ERROR] Failed to remove: %v\n", err)
		return
	}

	fmt.Printf("[INFO] %s removed successfully.\n", inst.Name)
}

func viewLogs() {
	inst := selectInstance("Select instance to view logs:", "")
	if inst == nil {
		return
	}

	logs, err := sdkClient.GetInstanceLogs(inst.ID)
	if err != nil {
		fmt.Printf("[ERROR] Failed to fetch logs: %v\n", err)
		return
	}

	fmt.Println("--- Logs Start ---")
	fmt.Print(logs)
	fmt.Println("--- Logs End ---")
}

func showInstance() {
	inst := selectInstance("Select instance to view details:", "")
	if inst == nil {
		return
	}

	// Fetch fresh details
	details, err := sdkClient.GetInstance(inst.ID)
	if err != nil {
		fmt.Printf("[ERROR] Failed to fetch details: %v\n", err)
		return
	}

	fmt.Printf("\nInstance Details\n")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("%-15s %v\n", "ID:", details.ID)
	fmt.Printf("%-15s %v\n", "Name:", details.Name)
	fmt.Printf("%-15s %v\n", "Status:", details.Status)
	fmt.Printf("%-15s %v\n", "Image:", details.Image)
	fmt.Printf("%-15s %v\n", "Ports:", details.Ports)
	fmt.Printf("%-15s %v\n", "Created At:", details.CreatedAt)
	fmt.Printf("%-15s %v\n", "Version:", details.Version)
	fmt.Printf("%-15s %v\n", "Container ID:", details.ContainerID)
	fmt.Println(strings.Repeat("-", 40))
}
func vpcMenu() {
	mode := ""
	prompt := &survey.Select{
		Message: "VPC Management",
		Options: []string{"List VPCs", "Create VPC", "Remove VPC", "Back"},
	}
	survey.AskOne(prompt, &mode)

	switch mode {
	case "List VPCs":
		vpcs, err := sdkClient.ListVPCs()
		if err != nil {
			fmt.Printf("[ERROR] Error: %v\n", err)
			return
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "NAME", "NETWORK ID"})
		for _, v := range vpcs {
			table.Append([]string{v.ID[:8], v.Name, v.NetworkID[:12]})
		}
		table.Render()

	case "Create VPC":
		name := ""
		survey.AskOne(&survey.Input{Message: "Enter VPC Name:"}, &name)
		if name == "" {
			return
		}
		vpc, err := sdkClient.CreateVPC(name)
		if err != nil {
			fmt.Printf("[ERROR] Error: %v\n", err)
			return
		}
		fmt.Printf("[SUCCESS] VPC %s created.\n", vpc.Name)

	case "Remove VPC":
		vpcs, _ := sdkClient.ListVPCs()
		if len(vpcs) == 0 {
			fmt.Println("No VPCs found.")
			return
		}
		var options []string
		for _, v := range vpcs {
			options = append(options, fmt.Sprintf("%s (%s)", v.Name, v.ID[:8]))
		}
		choice := ""
		survey.AskOne(&survey.Select{Message: "Select VPC to remove:", Options: options}, &choice)
		for _, v := range vpcs {
			if choice == fmt.Sprintf("%s (%s)", v.Name, v.ID[:8]) {
				if err := sdkClient.DeleteVPC(v.ID); err != nil {
					fmt.Printf("[ERROR] Error: %v\n", err)
				} else {
					fmt.Println("[SUCCESS] VPC removed.")
				}
				break
			}
		}
	}
}
