package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/poyrazk/thecloud/pkg/sdk"
	"gopkg.in/yaml.v3"
)

type CLIOptions struct {
	JSON     bool
	Output   string
	APIKey   string
	APIURL   string
	TenantID string
	Debug    bool
}

var opts CLIOptions

const errFmt = "Error: %v\n"

func createClient(o CLIOptions) *sdk.Client {
	cfg := loadFullConfig()

	key := o.APIKey
	if key == "" {
		key = os.Getenv("CLOUD_API_KEY")
	}
	if key == "" {
		key = loadConfigFile()
	}

	if key == "" {
		fmt.Println("[WARN] No API Key found. Run 'cloud auth create-demo <name>' to get one.")
		os.Exit(1)
	}

	apiURL := o.APIURL
	if apiURL == "http://localhost:8080" && cfg.APIURL != "" {
		apiURL = cfg.APIURL
	}

	client := sdk.NewClient(apiURL, key)

	tenant := o.TenantID
	if tenant == "" {
		tenant = os.Getenv("CLOUD_TENANT_ID")
	}
	if tenant == "" {
		tenant = cfg.Tenant
	}

	if tenant != "" {
		client.SetTenant(tenant)
	}

	if o.Debug || cfg.Debug {
		client.EnableDebug()
	}

	return client
}

func printJSON(data interface{}) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Println(string(b))
}

func printOutput(data interface{}) {
	output := opts.Output
	if output == "" {
		output = "table"
	}

	switch output {
	case "json":
		printJSON(data)
	case "yaml":
		b, err := yaml.Marshal(data)
		if err != nil {
			fmt.Printf("Error marshaling YAML: %v\n", err)
			return
		}
		fmt.Println(string(b))
	default:
		// For table output, fall back to JSON for now
		// Commands that use tablewriter handle their own output
		printJSON(data)
	}
}

func truncateID(id string) string {
	const n = 8
	if len(id) <= n {
		return id
	}
	return id[:n]
}

// listWithPagination is a generic helper for list commands that support pagination.
// It calls the paginated function if limit/offset are set, otherwise calls the regular function.
func listWithPagination[T any](
	regularFn func() ([]T, error),
	pagFn func(int, int) ([]T, *sdk.ListResponse[T], error),
	limit, offset int,
) ([]T, *sdk.ListResponse[T], error) {
	if limit > 0 || offset > 0 {
		return pagFn(limit, offset)
	}
	items, err := regularFn()
	return items, nil, err
}
