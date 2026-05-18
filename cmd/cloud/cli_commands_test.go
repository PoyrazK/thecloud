package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/pkg/sdk"
)

const (
	testCacheID  = "cache-1"
	testRedisMod = "redis-main"
	testQueueID  = "queue-1"
	testTopicID  = "topic-1"
	pathCaches   = "/caches/"
	pathQueues   = "/queues/"
	pathNotify   = "/notify/topics/"
)

func respondNoContent(w http.ResponseWriter) bool {
	w.WriteHeader(http.StatusNoContent)
	return true
}

func handleSecurityGroups(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/security-groups":
		resp := sdk.Response[[]sdk.SecurityGroup]{
			Data: []sdk.SecurityGroup{
				{
					ID:          "sg-1",
					VPCID:       "vpc-1",
					Name:        "default",
					Description: "default group",
					ARN:         "arn:sg:1",
					CreatedAt:   time.Now().UTC(),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/security-groups":
		resp := sdk.Response[sdk.SecurityGroup]{
			Data: sdk.SecurityGroup{
				ID:          "sg-2",
				VPCID:       "vpc-2",
				Name:        "web",
				Description: "web group",
				ARN:         "arn:sg:2",
				CreatedAt:   time.Now().UTC(),
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	}
	return false
}

func handleVPCs(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/vpcs":
		resp := sdk.Response[[]sdk.VPC]{
			Data: []sdk.VPC{
				{
					ID:        "vpc-1",
					Name:      "main",
					CIDRBlock: "10.0.0.0/16",
					NetworkID: "net-1",
					VXLANID:   1001,
					Status:    "available",
					CreatedAt: time.Now().UTC(),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/vpcs":
		resp := sdk.Response[sdk.VPC]{
			Data: sdk.VPC{
				ID:        "vpc-2",
				Name:      "demo",
				CIDRBlock: "10.1.0.0/16",
				NetworkID: "net-2",
				VXLANID:   1002,
				Status:    "available",
				CreatedAt: time.Now().UTC(),
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/vpcs/vpc-1/subnets":
		resp := sdk.Response[[]*sdk.Subnet]{
			Data: []*sdk.Subnet{
				{
					ID:        "subnet-1",
					VpcID:     "vpc-1",
					Name:      "public",
					CIDRBlock: "10.0.1.0/24",
					AZ:        "us-east-1a",
					GatewayIP: "10.0.1.1",
					Status:    "available",
					CreatedAt: time.Now().UTC(),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/vpcs/vpc-1/subnets":
		resp := sdk.Response[*sdk.Subnet]{
			Data: &sdk.Subnet{
				ID:        "subnet-2",
				VpcID:     "vpc-1",
				Name:      "private",
				CIDRBlock: "10.0.2.0/24",
				AZ:        "us-east-1b",
				GatewayIP: "10.0.2.1",
				Status:    "available",
				CreatedAt: time.Now().UTC(),
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/vpcs/vpc-1":
		return respondNoContent(w)
	case r.Method == http.MethodDelete && r.URL.Path == "/subnets/subnet-1":
		return respondNoContent(w)
	}
	return false
}

func handleVolumes(w http.ResponseWriter, r *http.Request) bool {
	volID := uuid.New()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/volumes":
		resp := sdk.Response[[]sdk.Volume]{
			Data: []sdk.Volume{
				{
					ID:        volID,
					Name:      "vol-1", // Named "vol-1" so short ID resolution by name works
					SizeGB:    20,
					Status:    "available",
					CreatedAt: time.Now().UTC(),
					UpdatedAt: time.Now().UTC(),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/volumes":
		resp := sdk.Response[sdk.Volume]{
			Data: sdk.Volume{
				ID:        volID,
				Name:      "data",
				SizeGB:    20,
				Status:    "available",
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/volumes/"):
		// Accept any volume ID prefix since resolveID may resolve short IDs to full UUID
		return respondNoContent(w)
	}
	return false
}

func handleCaches(w http.ResponseWriter, r *http.Request) bool {
	vpcID := "vpc-1"
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/caches":
		resp := sdk.Response[sdk.Cache]{
			Data: sdk.Cache{
				ID:        testCacheID,
				Name:      testRedisMod,
				Engine:    "redis",
				Version:   "7.2",
				Status:    "PROVISIONING",
				VpcID:     &vpcID,
				Port:      6379,
				MemoryMB:  128,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/caches":
		resp := sdk.Response[[]*sdk.Cache]{
			Data: []*sdk.Cache{
				{
					ID:       testCacheID,
					Name:     testRedisMod,
					Engine:   "redis",
					Version:  "7.2",
					Status:   "RUNNING",
					VpcID:    &vpcID,
					Port:     6379,
					MemoryMB: 128,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodGet && r.URL.Path == pathCaches+testCacheID:
		resp := sdk.Response[sdk.Cache]{
			Data: sdk.Cache{
				ID:       testCacheID,
				Name:     testRedisMod,
				Engine:   "redis",
				Version:  "7.2",
				Status:   "RUNNING",
				VpcID:    &vpcID,
				Port:     6379,
				MemoryMB: 128,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == pathCaches+testCacheID:
		return respondNoContent(w)
	case r.Method == http.MethodGet && r.URL.Path == pathCaches+testCacheID+"/connection":
		resp := sdk.Response[map[string]string]{
			Data: map[string]string{"connection_string": "redis://" + testCacheID + ":6379"},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodPost && r.URL.Path == pathCaches+testCacheID+"/flush":
		return respondNoContent(w)
	case r.Method == http.MethodGet && r.URL.Path == pathCaches+testCacheID+"/stats":
		resp := sdk.Response[sdk.CacheStats]{
			Data: sdk.CacheStats{
				UsedMemoryBytes:  1024,
				MaxMemoryBytes:   2048,
				ConnectedClients: 5,
				TotalKeys:        10,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	}
	return false
}

func handleQueues(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/queues":
		resp := sdk.Response[sdk.Queue]{
			Data: sdk.Queue{
				ID:     testQueueID,
				Name:   "jobs",
				ARN:    "arn:queue:1",
				Status: "ACTIVE",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/queues":
		resp := sdk.Response[[]sdk.Queue]{
			Data: []sdk.Queue{
				{
					ID:     testQueueID,
					Name:   "jobs",
					ARN:    "arn:queue:1",
					Status: "ACTIVE",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == pathQueues+testQueueID:
		return respondNoContent(w)
	case r.Method == http.MethodPost && r.URL.Path == pathQueues+testQueueID+"/messages":
		resp := sdk.Response[sdk.Message]{
			Data: sdk.Message{
				ID:            "msg-1",
				QueueID:       testQueueID,
				Body:          "hello",
				ReceiptHandle: "rh-1",
				CreatedAt:     time.Now().UTC(),
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodGet && r.URL.Path == pathQueues+testQueueID+"/messages":
		resp := sdk.Response[[]sdk.Message]{
			Data: []sdk.Message{
				{
					ID:            "msg-1",
					QueueID:       testQueueID,
					Body:          "hello",
					ReceiptHandle: "rh-1",
					CreatedAt:     time.Now().UTC(),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == pathQueues+testQueueID+"/messages/rh-1":
		return respondNoContent(w)
	case r.Method == http.MethodPost && r.URL.Path == pathQueues+testQueueID+"/purge":
		return respondNoContent(w)
	}
	return false
}

func handleNotify(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/notify/topics":
		resp := sdk.Response[sdk.Topic]{
			Data: sdk.Topic{
				ID:   testTopicID,
				Name: "updates",
				ARN:  "arn:topic:1",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/notify/topics":
		resp := sdk.Response[[]sdk.Topic]{
			Data: []sdk.Topic{{ID: testTopicID, Name: "updates", ARN: "arn:topic:1"}},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return true
	case r.Method == http.MethodPost && r.URL.Path == pathNotify+testTopicID+"/subscriptions":
		resp := sdk.Response[sdk.Subscription]{
			Data: sdk.Subscription{
				ID:       "sub-1",
				TopicID:  testTopicID,
				Protocol: "webhook",
				Endpoint: "https://example.com",
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return true
	case r.Method == http.MethodPost && r.URL.Path == pathNotify+testTopicID+"/publish":
		return respondNoContent(w)
	}
	return false
}

func setupAPIServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if handleSecurityGroups(w, r) ||
			handleVPCs(w, r) ||
			handleVolumes(w, r) ||
			handleCaches(w, r) ||
			handleQueues(w, r) ||
			handleNotify(w, r) {
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func setAPIContext(t *testing.T, server *httptest.Server) {
	t.Helper()

	oldURL := opts.APIURL
	oldKey := opts.APIKey
	oldJSON := opts.JSON

	opts.APIURL = server.URL
	opts.APIKey = "test-key"
	opts.JSON = false

	t.Cleanup(func() {
		opts.APIURL = oldURL
		opts.APIKey = oldKey
		opts.JSON = oldJSON
	})
}

func TestSGListCommandJSONOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	_ = sgListCmd.Flags().Set(flagVPCID, "vpc-1")
	opts.JSON = true
	t.Cleanup(func() {
		_ = sgListCmd.Flags().Set(flagVPCID, "")
		opts.JSON = false
	})

	out := captureStdout(t, func() {
		sgListCmd.Run(sgListCmd, nil)
	})
	if !strings.Contains(out, "\"id\": \"sg-1\"") {
		t.Fatalf("expected security group in output, got: %s", out)
	}
}

func TestSGCreateCommandMissingVPC(t *testing.T) {
	out := captureStdout(t, func() {
		sgCreateCmd.Run(sgCreateCmd, []string{"web"})
	})
	if !strings.Contains(out, "--vpc-id is required") {
		t.Fatalf("expected missing vpc-id error, got: %s", out)
	}
}

func TestVPCCreateCommandSuccess(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		vpcCreateCmd.Run(vpcCreateCmd, []string{"demo"})
	})
	if !strings.Contains(out, "VPC demo created successfully") {
		t.Fatalf("expected success message, got: %s", out)
	}
}

func TestVPCListCommandJSONOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	opts.JSON = true
	t.Cleanup(func() { opts.JSON = false })

	out := captureStdout(t, func() {
		vpcListCmd.Run(vpcListCmd, nil)
	})
	if !strings.Contains(out, "\"id\": \"vpc-1\"") {
		t.Fatalf("expected VPC in output, got: %s", out)
	}
}

func TestSubnetListCommandJSONOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	opts.JSON = true
	t.Cleanup(func() { opts.JSON = false })

	out := captureStdout(t, func() {
		subnetListCmd.Run(subnetListCmd, []string{"vpc-1"})
	})
	if !strings.Contains(out, "\"id\": \"subnet-1\"") {
		t.Fatalf("expected subnet in output, got: %s", out)
	}
}

func TestVolumeCreateCommandJSONOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	_ = volumeCreateCmd.Flags().Set("name", "data")
	_ = volumeCreateCmd.Flags().Set("size", "20")
	defer func() {
		_ = volumeCreateCmd.Flags().Set("name", "")
		_ = volumeCreateCmd.Flags().Set("size", "1")
	}()

	out := captureStdout(t, func() {
		volumeCreateCmd.Run(volumeCreateCmd, nil)
	})
	if !strings.Contains(out, "Volume created") {
		t.Fatalf("expected volume create output, got: %s", out)
	}
	if !strings.Contains(out, "\"name\": \"data\"") {
		t.Fatalf("expected volume json output, got: %s", out)
	}
}

func TestVolumeListCommandJSONOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	opts.JSON = true
	t.Cleanup(func() { opts.JSON = false })

	out := captureStdout(t, func() {
		volumeListCmd.Run(volumeListCmd, nil)
	})
	if !strings.Contains(out, "\"name\": \"vol-1\"") {
		t.Fatalf("expected volume list output, got: %s", out)
	}
}

func TestVolumeDeleteCommandSuccess(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		volumeDeleteCmd.Run(volumeDeleteCmd, []string{"vol-1"})
	})
	if !strings.Contains(out, "Volume vol-1 deleted") {
		t.Fatalf("expected volume delete output, got: %s", out)
	}
}

func TestVPCRmCommandSuccess(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		vpcRmCmd.Run(vpcRmCmd, []string{"vpc-1"})
	})
	if !strings.Contains(out, "VPC vpc-1 removed") {
		t.Fatalf("expected VPC remove output, got: %s", out)
	}
}

func TestSubnetDeleteCommandSuccess(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		subnetDeleteCmd.Run(subnetDeleteCmd, []string{"subnet-1"})
	})
	if !strings.Contains(out, "deleted successfully") {
		t.Fatalf("expected subnet delete output, got: %s", out)
	}
}

func TestCacheCreateCommandSuccess(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	_ = createCacheCmd.Flags().Set("name", testRedisMod)
	_ = createCacheCmd.Flags().Set("version", "7.2")
	_ = createCacheCmd.Flags().Set("memory", "128")
	_ = createCacheCmd.Flags().Set("vpc", "vpc-1")
	_ = createCacheCmd.Flags().Set("wait", "false")
	defer func() {
		_ = createCacheCmd.Flags().Set("name", "")
		_ = createCacheCmd.Flags().Set("version", "7.2")
		_ = createCacheCmd.Flags().Set("memory", "128")
		_ = createCacheCmd.Flags().Set("vpc", "")
		_ = createCacheCmd.Flags().Set("wait", "false")
	}()

	out := captureStdout(t, func() {
		createCacheCmd.Run(createCacheCmd, nil)
	})
	if !strings.Contains(out, "Cache created with ID: "+testCacheID) {
		t.Fatalf("expected cache create output, got: %s", out)
	}
}

func TestCacheListCommandOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		listCacheCmd.Run(listCacheCmd, nil)
	})
	if !strings.Contains(out, testCacheID) {
		t.Fatalf("expected cache list output, got: %s", out)
	}
}

func TestCacheShowCommandOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		getCacheCmd.Run(getCacheCmd, []string{testCacheID})
	})
	if !strings.Contains(out, "Name:      "+testRedisMod) {
		t.Fatalf("expected cache show output, got: %s", out)
	}
}

func TestCacheDeleteCommandOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		deleteCacheCmd.Run(deleteCacheCmd, []string{testCacheID})
	})
	if !strings.Contains(out, "Cache deleted successfully") {
		t.Fatalf("expected cache delete output, got: %s", out)
	}
}

func TestCacheConnectionCommandOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		connectionCacheCmd.Run(connectionCacheCmd, []string{testCacheID})
	})
	if !strings.Contains(out, "redis://"+testCacheID+":6379") {
		t.Fatalf("expected cache connection output, got: %s", out)
	}
}

func TestCacheStatsCommandOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		statsCacheCmd.Run(statsCacheCmd, []string{testCacheID})
	})
	if !strings.Contains(out, "Used Memory: 1.0 KB") {
		t.Fatalf("expected cache stats output, got: %s", out)
	}
}

func TestCacheFlushCommandOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	_ = flushCacheCmd.Flags().Set("yes", "true")
	t.Cleanup(func() {
		_ = flushCacheCmd.Flags().Set("yes", "false")
	})

	out := captureStdout(t, func() {
		flushCacheCmd.Run(flushCacheCmd, []string{testCacheID})
	})
	if !strings.Contains(out, "Cache flushed successfully") {
		t.Fatalf("expected cache flush output, got: %s", out)
	}
}

func TestQueueListCommandJSONOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	opts.JSON = true
	t.Cleanup(func() { opts.JSON = false })

	out := captureStdout(t, func() {
		listQueuesCmd.Run(listQueuesCmd, nil)
	})
	if !strings.Contains(out, "\"id\": \""+testQueueID+"\"") {
		t.Fatalf("expected queue list output, got: %s", out)
	}
}

func TestQueueCreateCommandOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		createQueueCmd.Run(createQueueCmd, []string{"jobs"})
	})
	if !strings.Contains(out, "Queue created successfully") {
		t.Fatalf("expected queue create output, got: %s", out)
	}
}

func TestQueueDeleteCommandOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		deleteQueueCmd.Run(deleteQueueCmd, []string{testQueueID})
	})
	if !strings.Contains(out, "Queue deleted successfully") {
		t.Fatalf("expected queue delete output, got: %s", out)
	}
}

func TestQueueSendMessageOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		sendMessageCmd.Run(sendMessageCmd, []string{testQueueID, "hello"})
	})
	if !strings.Contains(out, "Message sent") {
		t.Fatalf("expected send message output, got: %s", out)
	}
}

func TestQueueReceiveMessagesJSONOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	_ = receiveMessagesCmd.Flags().Set("max", "1")
	opts.JSON = true
	t.Cleanup(func() {
		_ = receiveMessagesCmd.Flags().Set("max", "1")
		opts.JSON = false
	})

	out := captureStdout(t, func() {
		receiveMessagesCmd.Run(receiveMessagesCmd, []string{testQueueID})
	})
	if !strings.Contains(out, "\"id\": \"msg-1\"") {
		t.Fatalf("expected receive messages output, got: %s", out)
	}
}

func TestQueueAckMessageOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		ackMessageCmd.Run(ackMessageCmd, []string{testQueueID, "rh-1"})
	})
	if !strings.Contains(out, "Message acknowledged") {
		t.Fatalf("expected ack output, got: %s", out)
	}
}

func TestQueuePurgeOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		purgeQueueCmd.Run(purgeQueueCmd, []string{testQueueID})
	})
	if !strings.Contains(out, "Queue purged") {
		t.Fatalf("expected queue purge output, got: %s", out)
	}
}

func TestNotifyCreateTopicOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		createTopicCmd.Run(createTopicCmd, []string{"updates"})
	})
	if !strings.Contains(out, "Topic created") {
		t.Fatalf("expected create topic output, got: %s", out)
	}
}

func TestNotifyListTopicsOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		listTopicsCmd.Run(listTopicsCmd, nil)
	})
	if !strings.Contains(out, "updates") {
		t.Fatalf("expected list topics output, got: %s", out)
	}
}

func TestNotifySubscribeOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	_ = subscribeCmd.Flags().Set("protocol", "webhook")
	_ = subscribeCmd.Flags().Set("endpoint", "https://example.com")
	defer func() {
		_ = subscribeCmd.Flags().Set("endpoint", "")
	}()

	out := captureStdout(t, func() {
		subscribeCmd.Run(subscribeCmd, []string{testTopicID})
	})
	if !strings.Contains(out, "Subscription created") {
		t.Fatalf("expected subscribe output, got: %s", out)
	}
}

func TestNotifyPublishOutput(t *testing.T) {
	server := setupAPIServer(t)
	defer server.Close()
	setAPIContext(t, server)

	out := captureStdout(t, func() {
		publishCmd.Run(publishCmd, []string{testTopicID, "hello"})
	})
	if !strings.Contains(out, "Message published") {
		t.Fatalf("expected publish output, got: %s", out)
	}
}
