// Package main provides the cloud CLI entrypoint.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/spf13/cobra"
)

var functionCmd = &cobra.Command{
	Use:   "function",
	Short: "Manage CloudFunctions",
}

var createFnCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new function",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		runtime, _ := cmd.Flags().GetString("runtime")
		handler, _ := cmd.Flags().GetString("handler")
		codePath, _ := cmd.Flags().GetString("code")

		code, err := os.ReadFile(filepath.Clean(codePath))
		if err != nil {
			return fmt.Errorf("failed to read code file: %w", err)
		}

		client := createClient(opts)
		fn, err := client.CreateFunction(name, runtime, handler, code)
		if err != nil {
			return err
		}

		fmt.Printf("Function %s created successfully (ID: %s)\n", fn.Name, fn.ID)
		return nil
	},
}

var listFnCmd = &cobra.Command{
	Use:   "list",
	Short: "List all functions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := createClient(opts)

		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")

		var functions []*sdk.Function
		var meta *sdk.ListResponse[sdk.Function]

		if limit > 0 || offset > 0 {
			var err error
			functions, meta, err = client.ListFunctionsWithPagination(limit, offset)
			if err != nil {
				return err
			}
		} else {
			var err error
			functions, err = client.ListFunctions()
			if err != nil {
				return err
			}
		}

		if len(functions) == 0 {
			fmt.Println("No functions found.")
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"ID", "Name", "Runtime", "Status", "Created At"})
		for _, f := range functions {
			table.Append([]string{f.ID, f.Name, f.Runtime, f.Status, f.CreatedAt.Format("2006-01-02 15:04:05")})
		}
		table.Render()

		if meta != nil {
			fmt.Printf("\nShowing %d of %d total", len(functions), meta.TotalCount)
			if meta.HasMore {
				fmt.Print(" (more available)")
			}
			fmt.Println()
		}
		return nil
	},
}

var invokeFnCmd = &cobra.Command{
	Use:   "invoke [name/id]",
	Short: "Invoke a function",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		payloadStr, _ := cmd.Flags().GetString("payload")
		payloadFile, _ := cmd.Flags().GetString("payload-file")
		async, _ := cmd.Flags().GetBool("async")

		var payload []byte
		var err error
		if payloadFile != "" {
			payload, err = os.ReadFile(filepath.Clean(payloadFile))
			if err != nil {
				return err
			}
		} else {
			payload = []byte(payloadStr)
		}

		client := createClient(opts)

		// Map name to ID if needed
		targetID := id
		functions, err := client.ListFunctions()
		if err == nil {
			for _, f := range functions {
				if f.Name == id {
					targetID = f.ID
					break
				}
			}
		}

		invocation, err := client.InvokeFunction(targetID, payload, async)
		if err != nil {
			return err
		}

		if async {
			fmt.Printf("Invocation started (ID: %s)\n", invocation.ID)
		} else {
			fmt.Printf("Status: %s\n", invocation.Status)
			fmt.Printf("Exit Code: %d\n", invocation.StatusCode)
			fmt.Printf("Duration: %dms\n", invocation.DurationMs)
			fmt.Println("\nLogs:")
			fmt.Println(invocation.Logs)
		}
		return nil
	},
}

var logsFnCmd = &cobra.Command{
	Use:   "logs [name/id]",
	Short: "Get recent logs for a function",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		client := createClient(opts)

		targetID := id
		functions, err := client.ListFunctions()
		if err == nil {
			for _, f := range functions {
				if f.Name == id {
					targetID = f.ID
					break
				}
			}
		}

		invocations, err := client.GetFunctionLogs(targetID)
		if err != nil {
			return err
		}

		for _, i := range invocations {
			fmt.Printf("--- Invocation %s (%s) ---\n", i.ID, i.StartedAt.Format("15:04:05"))
			fmt.Printf("Status: %s, Exit Code: %d, Duration: %dms\n", i.Status, i.StatusCode, i.DurationMs)
			fmt.Println(i.Logs)
			fmt.Println()
		}
		return nil
	},
}

var rmFnCmd = &cobra.Command{
	Use:   "rm [name/id]",
	Short: "Remove a function",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		client := createClient(opts)

		targetID := id
		functions, err := client.ListFunctions()
		if err == nil {
			for _, f := range functions {
				if f.Name == id {
					targetID = f.ID
					break
				}
			}
		}

		if err := client.DeleteFunction(targetID); err != nil {
			return err
		}
		fmt.Printf("Function %s removed.\n", id)
		return nil
	},
}

var updateFnCmd = &cobra.Command{
	Use:   "update [name/id]",
	Short: "Update a function's configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		client := createClient(opts)

		targetID := id
		functions, err := client.ListFunctions()
		if err == nil {
			for _, f := range functions {
				if f.Name == id {
					targetID = f.ID
					break
				}
			}
		}

		handler, _ := cmd.Flags().GetString("handler")
		timeout, _ := cmd.Flags().GetInt("timeout")
		memoryMB, _ := cmd.Flags().GetInt("memory")
		envVarsStr, _ := cmd.Flags().GetStringSlice("env")
		status, _ := cmd.Flags().GetString("status")

		var handlerPtr *string
		if handler != "" {
			handlerPtr = &handler
		}
		var timeoutPtr *int
		if cmd.Flags().Changed("timeout") {
			timeoutPtr = &timeout
		}
		var memoryPtr *int
		if cmd.Flags().Changed("memory") {
			memoryPtr = &memoryMB
		}

		var envVars []*sdk.EnvVar
		for _, e := range envVarsStr {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid env var format: %q, expected KEY=VALUE", e)
			}
			envVar := &sdk.EnvVar{Key: parts[0]}
			if strings.HasPrefix(parts[1], "@") {
				envVar.SecretRef = parts[1] // e.g. "@my-api-key"
			} else {
				envVar.Value = parts[1]
			}
			envVars = append(envVars, envVar)
		}

		fn, err := client.UpdateFunction(targetID, &sdk.FunctionUpdateRequest{
			Handler:  handlerPtr,
			Timeout:  timeoutPtr,
			MemoryMB: memoryPtr,
			EnvVars:  envVars,
			Status:   status,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Function %s updated (ID: %s)\n", fn.Name, fn.ID)
		return nil
	},
}

func init() {
	createFnCmd.Flags().StringP("name", "n", "", "Function name")
	createFnCmd.Flags().StringP("runtime", "r", "nodejs20", "Runtime (nodejs20, python312, go122, ruby33, java21)")
	createFnCmd.Flags().StringP("handler", "H", "index.handler", "Handler name")
	createFnCmd.Flags().StringP("code", "c", "", "Path to code zip file")
	_ = createFnCmd.MarkFlagRequired("name")
	_ = createFnCmd.MarkFlagRequired("code")

	invokeFnCmd.Flags().StringP("payload", "p", "{}", "JSON payload")
	invokeFnCmd.Flags().StringP("payload-file", "f", "", "Path to payload file")
	invokeFnCmd.Flags().BoolP("async", "a", false, "Invoke asynchronously")

	updateFnCmd.Flags().StringP("handler", "H", "", "Handler name")
	updateFnCmd.Flags().Int("timeout", 0, "Timeout in seconds (1-900)")
	updateFnCmd.Flags().Int("memory", 0, "Memory in MB (64-10240)")
	updateFnCmd.Flags().StringSlice("env", []string{}, "Environment variable KEY=VALUE")
	updateFnCmd.Flags().String("status", "", "Function status (e.g., paused, running)")

	listFnCmd.Flags().Int("limit", 0, "Maximum number of results (0 = use server default)")
	listFnCmd.Flags().Int("offset", 0, "Number of results to skip")

	functionCmd.AddCommand(createFnCmd)
	functionCmd.AddCommand(listFnCmd)
	functionCmd.AddCommand(invokeFnCmd)
	functionCmd.AddCommand(logsFnCmd)
	functionCmd.AddCommand(rmFnCmd)
	functionCmd.AddCommand(updateFnCmd)
}
