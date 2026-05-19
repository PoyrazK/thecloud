package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientListRouteTables(t *testing.T) {
	t.Parallel()
	vpcID := "vpc-123"
	expectedRTs := []RouteTable{
		{ID: "rt-1", VPCID: vpcID, Name: "main-rt", IsMain: true},
		{ID: "rt-2", VPCID: vpcID, Name: "custom-rt", IsMain: false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/route-tables", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, vpcID, r.URL.Query().Get("vpc_id"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response[[]RouteTable]{Data: expectedRTs})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	rts, err := client.ListRouteTables(vpcID)

	require.NoError(t, err)
	assert.Len(t, rts, 2)
	assert.Equal(t, expectedRTs[0].ID, rts[0].ID)
	assert.True(t, rts[0].IsMain)
}

func TestClientCreateRouteTable(t *testing.T) {
	t.Parallel()
	vpcID := "vpc-123"
	expectedRT := RouteTable{
		ID:     "rt-456",
		VPCID:  vpcID,
		Name:   "new-rt",
		IsMain: false,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/route-tables", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, vpcID, req["vpc_id"])
		assert.Equal(t, "new-rt", req["name"])
		assert.Equal(t, false, req["is_main"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response[RouteTable]{Data: expectedRT})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	rt, err := client.CreateRouteTable(vpcID, "new-rt", false)

	require.NoError(t, err)
	assert.NotNil(t, rt)
	assert.Equal(t, expectedRT.ID, rt.ID)
}

func TestClientGetRouteTable(t *testing.T) {
	t.Parallel()
	id := "rt-123"
	expectedRT := RouteTable{
		ID:    id,
		VPCID: "vpc-456",
		Name:  "test-rt",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/route-tables/"+id, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response[RouteTable]{Data: expectedRT})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	rt, err := client.GetRouteTable(id)

	require.NoError(t, err)
	assert.NotNil(t, rt)
	assert.Equal(t, expectedRT.ID, rt.ID)
}

func TestClientDeleteRouteTable(t *testing.T) {
	t.Parallel()
	id := "rt-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/route-tables/"+id, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.DeleteRouteTable(id)

	require.NoError(t, err)
}

func strPtr(s string) *string { return &s }

func TestClientAddRoute(t *testing.T) {
	t.Parallel()
	rtID := "rt-123"
	expectedRoute := Route{
		ID:              "route-456",
		RouteTableID:    rtID,
		DestinationCIDR: "10.0.0.0/16",
		TargetType:      RouteTargetIGW,
		TargetID:        strPtr("igw-789"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/route-tables/"+rtID+"/routes", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "10.0.0.0/16", req["destination_cidr"])
		assert.Equal(t, "igw", req["target_type"])
		assert.Equal(t, "igw-789", req["target_id"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response[Route]{Data: expectedRoute})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	route, err := client.AddRoute(rtID, "10.0.0.0/16", RouteTargetIGW, "igw-789")

	require.NoError(t, err)
	assert.NotNil(t, route)
	assert.Equal(t, expectedRoute.ID, route.ID)
	assert.Equal(t, expectedRoute.DestinationCIDR, route.DestinationCIDR)
}

func TestClientAddRouteWithoutTargetID(t *testing.T) {
	t.Parallel()
	rtID := "rt-123"
	expectedRoute := Route{
		ID:              "route-456",
		RouteTableID:    rtID,
		DestinationCIDR: "0.0.0.0/0",
		TargetType:      RouteTargetLocal,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/route-tables/"+rtID+"/routes", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "0.0.0.0/0", req["destination_cidr"])
		assert.Equal(t, "local", req["target_type"])
		_, hasTargetID := req["target_id"]
		assert.False(t, hasTargetID, "target_id should not be present when empty")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response[Route]{Data: expectedRoute})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	route, err := client.AddRoute(rtID, "0.0.0.0/0", RouteTargetLocal, "")

	require.NoError(t, err)
	assert.NotNil(t, route)
	assert.Equal(t, expectedRoute.TargetType, route.TargetType)
}

func TestClientRemoveRoute(t *testing.T) {
	t.Parallel()
	rtID := "rt-123"
	routeID := "route-456"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/route-tables/"+rtID+"/routes/"+routeID, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.RemoveRoute(rtID, routeID)

	require.NoError(t, err)
}

func TestClientAssociateSubnet(t *testing.T) {
	t.Parallel()
	rtID := "rt-123"
	subnetID := "subnet-456"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/route-tables/"+rtID+"/associations", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req map[string]string
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, subnetID, req["subnet_id"])

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.AssociateSubnet(rtID, subnetID)

	require.NoError(t, err)
}

func TestClientDisassociateSubnet(t *testing.T) {
	t.Parallel()
	rtID := "rt-123"
	subnetID := "subnet-456"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/route-tables/"+rtID+"/associations/"+subnetID, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.DisassociateSubnet(rtID, subnetID)

	require.NoError(t, err)
}

func TestClientListRouteTablesError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	_, err := client.ListRouteTables("vpc-123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientCreateRouteTableError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid vpc_id"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	_, err := client.CreateRouteTable("invalid-uuid", "my-rt", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientGetRouteTableError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"route table not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	_, err := client.GetRouteTable("nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientDeleteRouteTableError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"cannot delete main route table"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.DeleteRouteTable("rt-123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientAddRouteError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid CIDR format"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	_, err := client.AddRoute("rt-123", "invalid", RouteTargetIGW, "igw-789")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientRemoveRouteError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"route not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.RemoveRoute("rt-123", "nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientAssociateSubnetError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"subnet already associated"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.AssociateSubnet("rt-123", "subnet-456")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}

func TestClientDisassociateSubnetError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"association not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	err := client.DisassociateSubnet("rt-123", "subnet-456")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api error")
}
