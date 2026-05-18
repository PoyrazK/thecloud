package ccm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/poyrazk/thecloud/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudprovider "k8s.io/cloud-provider"
)

func TestInstancesV2(t *testing.T) {
	// Setup a real HTTP server to act as the Cloud API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/instances":
			// List instances for resolveID lookup
			instances := []sdk.Instance{
				{ID: "inst-123", Name: "test-node", PrivateIP: "10.0.0.5", Status: "RUNNING", InstanceType: "standard-2"},
				{ID: "inst-456", Name: "stopped-node", Status: StatusStopped},
			}
			if err := json.NewEncoder(w).Encode(sdk.Response[[]sdk.Instance]{Data: instances}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/instances/inst-123":
			inst := sdk.Instance{
				ID:           "inst-123",
				Name:         "test-node",
				PrivateIP:    "10.0.0.5",
				Status:       "RUNNING",
				InstanceType: "standard-2",
			}
			if err := json.NewEncoder(w).Encode(sdk.Response[sdk.Instance]{Data: inst}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/instances/inst-456":
			inst := sdk.Instance{
				ID:     "inst-456",
				Status: StatusStopped,
			}
			if err := json.NewEncoder(w).Encode(sdk.Response[sdk.Instance]{Data: inst}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `{"error": {"message": "not found"}}`)
		}
	}))
	defer server.Close()

	client := sdk.NewClient(server.URL, "test-key")
	insts := newInstancesV2(client)

	type testCase struct {
		name           string
		nodeName       string
		testFn         func(context.Context, *v1.Node) (interface{}, error)
		expectedResult interface{}
		expectError    bool
	}

	tests := []testCase{
		{
			name:     "InstanceMetadata_Success",
			nodeName: "test-node",
			testFn: func(ctx context.Context, n *v1.Node) (interface{}, error) {
				return insts.InstanceMetadata(ctx, n)
			},
			expectedResult: &cloudprovider.InstanceMetadata{
				ProviderID:   "thecloud://inst-123",
				InstanceType: "standard-2",
				NodeAddresses: []v1.NodeAddress{
					{Type: v1.NodeInternalIP, Address: "10.0.0.5"},
					{Type: v1.NodeHostName, Address: "test-node"},
				},
				Zone:   "local",
				Region: "local",
			},
		},
		{
			name:     "InstanceExists_True",
			nodeName: "test-node",
			testFn: func(ctx context.Context, n *v1.Node) (interface{}, error) {
				return insts.InstanceExists(ctx, n)
			},
			expectedResult: true,
		},
		{
			name:     "InstanceExists_False",
			nodeName: "missing-node",
			testFn: func(ctx context.Context, n *v1.Node) (interface{}, error) {
				return insts.InstanceExists(ctx, n)
			},
			expectedResult: false,
		},
		{
			name:     "InstanceShutdown_False",
			nodeName: "test-node",
			testFn: func(ctx context.Context, n *v1.Node) (interface{}, error) {
				return insts.InstanceShutdown(ctx, n)
			},
			expectedResult: false,
		},
		{
			name:     "InstanceShutdown_True",
			nodeName: "stopped-node",
			testFn: func(ctx context.Context, n *v1.Node) (interface{}, error) {
				return insts.InstanceShutdown(ctx, n)
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: tt.nodeName}}
			res, err := tt.testFn(context.Background(), node)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, res)
			}
		})
	}
}
