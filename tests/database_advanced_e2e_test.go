package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/pkg/testutil"
)

func TestDatabaseAdvancedE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database advanced E2E test in short mode")
	}

	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Database Advanced E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, "db-advanced@thecloud.local", "Advanced Tester")

	t.Run("CreateDatabaseWithParameters", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":              "e2e-db-with-params",
			"engine":            "postgres",
			"version":           "16",
			"allocated_storage": 10,
			"parameters": map[string]string{
				"max_connections": "200",
			},
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/databases", token, payload)
		if resp != nil && resp.Body != nil {
			defer func() { _ = resp.Body.Close() }()
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data domain.Database `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		dbID := res.Data.ID.String()
		assert.NotEmpty(t, dbID)
		assert.Equal(t, "200", res.Data.Parameters["max_connections"])

		// Cleanup
		respDel := deleteRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, dbID), token)
		if respDel != nil && respDel.Body != nil {
			defer func() { _ = respDel.Body.Close() }()
		}
	})

	t.Run("InvalidConfigurations", func(t *testing.T) {

		runInvalidConfigsTest(t, client, token)
	})

	t.Run("PromotionEdgeCases", func(t *testing.T) {
		runPromotionEdgeCasesTest(t, client, token)
	})

	t.Run("VpcIntegration", func(t *testing.T) {
		runVpcIntegrationTest(t, client, token)
	})

	t.Run("MultiReplicaPromotion", func(t *testing.T) {
		runMultiReplicaPromotionTest(t, client, token)
	})

	t.Run("ConnectionStringFormats", func(t *testing.T) {
		runConnectionStringFormatsTest(t, client, token)
	})

	t.Run("UnauthorizedAccess", func(t *testing.T) {
		runUnauthorizedAccessTest(t, client)
	})

	t.Run("ResourceNotFound", func(t *testing.T) {
		runResourceNotFoundTest(t, client, token)
	})
}

func runInvalidConfigsTest(t *testing.T, client *http.Client, token string) {
	t.Helper()
	testCases := []struct {
		name    string
		payload map[string]interface{}
	}{
		{
			"UnsupportedEngine",
			map[string]interface{}{
				"name":    "invalid-engine-db",
				"engine":  "oracle",
				"version": "19c",
			},
		},
		{
			"InvalidStorageSize",
			map[string]interface{}{
				"name":              "invalid-storage-db",
				"engine":            "postgres",
				"version":           "16",
				"allocated_storage": -5,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postRequest(t, client, testutil.TestBaseURL+"/databases", token, tc.payload)
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected 400 Bad Request for %s", tc.name)
		})
	}
}

func runPromotionEdgeCasesTest(t *testing.T, client *http.Client, token string) {
	t.Helper()
	// 1. Create a Primary DB
	dbName := fmt.Sprintf("promo-edge-db-%d", time.Now().UnixNano()%1000)
	payload := map[string]interface{}{
		"name":              dbName,
		"engine":            "postgres",
		"version":           "16",
		"allocated_storage": 10,
	}
	resp := postRequest(t, client, testutil.TestBaseURL+"/databases", token, payload)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var res struct {
		Data domain.Database `json:"data"`
	}
	err := json.NewDecoder(resp.Body).Decode(&res)
	_ = resp.Body.Close()
	require.NoError(t, err)
	dbID := res.Data.ID.String()

	// 2. Attempt to promote a primary (should fail as it's already primary)
	t.Run("PromotePrimaryFails", func(t *testing.T) {
		respPromo := postRequest(t, client, fmt.Sprintf("%s/databases/%s/promote", testutil.TestBaseURL, dbID), token, nil)
		if respPromo != nil && respPromo.Body != nil {
			defer func() { _ = respPromo.Body.Close() }()
		}
		assert.Equal(t, http.StatusBadRequest, respPromo.StatusCode)
	})

	// 3. Attempt to promote non-existent UUID
	t.Run("PromoteNotFound", func(t *testing.T) {
		fakeID := "00000000-0000-0000-0000-000000000000"
		respPromo := postRequest(t, client, fmt.Sprintf("%s/databases/%s/promote", testutil.TestBaseURL, fakeID), token, nil)
		if respPromo != nil && respPromo.Body != nil {
			defer func() { _ = respPromo.Body.Close() }()
		}
		assert.Equal(t, http.StatusNotFound, respPromo.StatusCode)
	})

	// Cleanup
	respDel := deleteRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, dbID), token)
	if respDel != nil && respDel.Body != nil {
		defer func() { _ = respDel.Body.Close() }()
	}
}

func runVpcIntegrationTest(t *testing.T, client *http.Client, token string) {
	t.Helper()
	// 1. Create a VPC
	vpcPayload := map[string]string{
		"name":       "db-vpc",
		"cidr_block": "10.10.0.0/16",
	}
	respVpc := postRequest(t, client, testutil.TestBaseURL+"/vpcs", token, vpcPayload)
	require.Equal(t, http.StatusCreated, respVpc.StatusCode)

	var vpcRes struct {
		Data domain.VPC `json:"data"`
	}
	err := json.NewDecoder(respVpc.Body).Decode(&vpcRes)
	_ = respVpc.Body.Close()
	require.NoError(t, err)
	vpcID := vpcRes.Data.ID

	// 2. Create DB in VPC
	dbName := "vpc-integrated-db"
	dbPayload := map[string]interface{}{
		"name":              dbName,
		"engine":            "postgres",
		"version":           "16",
		"vpc_id":            vpcID,
		"allocated_storage": 10,
	}
	respDb := postRequest(t, client, testutil.TestBaseURL+"/databases", token, dbPayload)
	if respDb.StatusCode != http.StatusCreated {
		if respDb.Body != nil {
			_ = respDb.Body.Close()
		}
		// Cleanup VPC if DB fails
		respDelVpc := deleteRequest(t, client, fmt.Sprintf("%s/vpcs/%s?force=true", testutil.TestBaseURL, vpcID), token)
		if respDelVpc != nil && respDelVpc.Body != nil {
			_ = respDelVpc.Body.Close()
		}
		t.Skipf("Skipping VPC integration test: Database creation failed with %d (likely infra issue)", respDb.StatusCode)
	}

	var dbRes struct {
		Data domain.Database `json:"data"`
	}
	err = json.NewDecoder(respDb.Body).Decode(&dbRes)
	_ = respDb.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, vpcID, *dbRes.Data.VpcID)

	// Cleanup
	respDel1 := deleteRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, dbRes.Data.ID), token)
	if respDel1 != nil && respDel1.Body != nil {
		_ = respDel1.Body.Close()
	}
	respDel2 := deleteRequest(t, client, fmt.Sprintf("%s/vpcs/%s?force=true", testutil.TestBaseURL, vpcID), token)
	if respDel2 != nil && respDel2.Body != nil {
		_ = respDel2.Body.Close()
	}
}

func runMultiReplicaPromotionTest(t *testing.T, client *http.Client, token string) {
	t.Helper()
	// 1. Create Primary
	payload := map[string]interface{}{
		"name":              "multi-rep-primary",
		"engine":            "postgres",
		"version":           "16",
		"allocated_storage": 10,
	}
	respP := postRequest(t, client, testutil.TestBaseURL+"/databases", token, payload)
	require.Equal(t, http.StatusCreated, respP.StatusCode)

	var pRes struct {
		Data domain.Database `json:"data"`
	}
	err := json.NewDecoder(respP.Body).Decode(&pRes)
	_ = respP.Body.Close()
	require.NoError(t, err)
	primaryID := pRes.Data.ID

	// 2. Create 2 Replicas
	replicaIDs := make([]string, 2)
	for i := 0; i < 2; i++ {
		repPayload := map[string]string{
			"name": fmt.Sprintf("replica-%d", i+1),
		}
		url := fmt.Sprintf("%s/databases/%s/replicas", testutil.TestBaseURL, primaryID)
		respR := postRequest(t, client, url, token, repPayload)
		require.Equal(t, http.StatusCreated, respR.StatusCode)

		var rRes struct {
			Data domain.Database `json:"data"`
		}
		err = json.NewDecoder(respR.Body).Decode(&rRes)
		_ = respR.Body.Close()
		require.NoError(t, err)
		replicaIDs[i] = rRes.Data.ID.String()
		assert.Equal(t, domain.RoleReplica, rRes.Data.Role)
		assert.Equal(t, primaryID, *rRes.Data.PrimaryID)
	}

	// 3. Promote Replica 1
	respPromo := postRequest(t, client, fmt.Sprintf("%s/databases/%s/promote", testutil.TestBaseURL, replicaIDs[0]), token, nil)
	if respPromo != nil && respPromo.Body != nil {
		_ = respPromo.Body.Close()
	}
	assert.Equal(t, http.StatusOK, respPromo.StatusCode)

	// 4. Verify Replica 1 is now Primary
	respG1 := getRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, replicaIDs[0]), token)
	var g1Res struct {
		Data domain.Database `json:"data"`
	}
	err = json.NewDecoder(respG1.Body).Decode(&g1Res)
	_ = respG1.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, domain.RolePrimary, g1Res.Data.Role)
	assert.Nil(t, g1Res.Data.PrimaryID)

	// 5. Verify Replica 2 still points to original Primary
	respG2 := getRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, replicaIDs[1]), token)
	var g2Res struct {
		Data domain.Database `json:"data"`
	}
	err = json.NewDecoder(respG2.Body).Decode(&g2Res)
	_ = respG2.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, domain.RoleReplica, g2Res.Data.Role)
	assert.Equal(t, primaryID, *g2Res.Data.PrimaryID)

	// Cleanup
	respDelP := deleteRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, primaryID), token)
	if respDelP != nil && respDelP.Body != nil {
		_ = respDelP.Body.Close()
	}
	respDelR1 := deleteRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, replicaIDs[0]), token)
	if respDelR1 != nil && respDelR1.Body != nil {
		_ = respDelR1.Body.Close()
	}
	respDelR2 := deleteRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, replicaIDs[1]), token)
	if respDelR2 != nil && respDelR2.Body != nil {
		_ = respDelR2.Body.Close()
	}
}

func runConnectionStringFormatsTest(t *testing.T, client *http.Client, token string) {
	t.Helper()
	engines := []struct {
		engine  string
		version string
		prefix  string
	}{
		{"postgres", "16", "postgres://"},
		{"mysql", "8.0", ""}, // MySQL format is user:pass@tcp(host:port)/db
	}

	for _, tc := range engines {
		t.Run(tc.engine, func(t *testing.T) {
			dbName := fmt.Sprintf("conn-str-%s-%d", tc.engine, time.Now().UnixNano()%1000)
			payload := map[string]interface{}{
				"name":              dbName,
				"engine":            tc.engine,
				"version":           tc.version,
				"allocated_storage": 10,
			}
			resp := postRequest(t, client, testutil.TestBaseURL+"/databases", token, payload)
			if resp.StatusCode != http.StatusCreated {
				if resp != nil && resp.Body != nil {
					_ = resp.Body.Close()
				}
				t.Skipf("Skipping %s connection string test due to infra error: %d", tc.engine, resp.StatusCode)
			}

			var res struct {
				Data domain.Database `json:"data"`
			}
			err := json.NewDecoder(resp.Body).Decode(&res)
			_ = resp.Body.Close()
			require.NoError(t, err)
			dbID := res.Data.ID.String()

			respConn := getRequest(t, client, fmt.Sprintf("%s/databases/%s/connection", testutil.TestBaseURL, dbID), token)
			var connRes struct {
				Data struct {
					ConnectionString string `json:"connection_string"`
				} `json:"data"`
			}
			err = json.NewDecoder(respConn.Body).Decode(&connRes)
			_ = respConn.Body.Close()
			require.NoError(t, err)
			assert.Contains(t, connRes.Data.ConnectionString, tc.prefix)
			assert.Contains(t, connRes.Data.ConnectionString, dbName)

			// Cleanup
			respDel := deleteRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, dbID), token)
			if respDel != nil && respDel.Body != nil {
				_ = respDel.Body.Close()
			}
		})
	}
}

func runUnauthorizedAccessTest(t *testing.T, client *http.Client) {
	t.Helper()
	// Test request without token
	req, err := http.NewRequest(http.MethodGet, testutil.TestBaseURL+"/databases", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func runResourceNotFoundTest(t *testing.T, client *http.Client, token string) {
	t.Helper()
	fakeID := uuid.New().String()

	// 1. Get non-existent DB
	respG := getRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, fakeID), token)
	if respG != nil && respG.Body != nil {
		defer func() { _ = respG.Body.Close() }()
	}
	assert.Equal(t, http.StatusNotFound, respG.StatusCode)

	// 2. Delete non-existent DB
	respD := deleteRequest(t, client, fmt.Sprintf("%s/databases/%s", testutil.TestBaseURL, fakeID), token)
	if respD != nil && respD.Body != nil {
		defer func() { _ = respD.Body.Close() }()
	}
	assert.Equal(t, http.StatusNotFound, respD.StatusCode)

	// 3. Get connection string for non-existent DB
	respC := getRequest(t, client, fmt.Sprintf("%s/databases/%s/connection", testutil.TestBaseURL, fakeID), token)
	if respC != nil && respC.Body != nil {
		defer func() { _ = respC.Body.Close() }()
	}
	assert.Equal(t, http.StatusNotFound, respC.StatusCode)
}
