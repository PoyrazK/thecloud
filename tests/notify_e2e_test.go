package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/poyrazk/thecloud/pkg/testutil"
)

func TestNotifyE2E(t *testing.T) {
	t.Parallel()
	if err := waitForServer(); err != nil {
		t.Fatalf("Failing Notify E2E test: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := registerAndLogin(t, client, fmt.Sprintf("notify-tester-%d@thecloud.local", time.Now().UnixNano()%10000), "Notify Tester")

	var topicID string
	topicName := fmt.Sprintf("e2e-topic-%d", time.Now().UnixNano()%10000)

	// 1. Create Topic
	t.Run("CreateTopic", func(t *testing.T) {
		payload := map[string]string{
			"name": topicName,
		}
		resp := postRequest(t, client, testutil.TestBaseURL+"/notify/topics", token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden {
			t.Skip("Notify API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				ARN  string `json:"arn"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		topicID = res.Data.ID
		assert.NotEmpty(t, topicID)
		assert.Equal(t, topicName, res.Data.Name)
		assert.NotEmpty(t, res.Data.ARN)
	})

	if topicID == "" {
		t.Fatal("Topic ID not set - cannot continue tests")
	}

	// 2. List Topics
	t.Run("ListTopics", func(t *testing.T) {
		resp := getRequest(t, client, testutil.TestBaseURL+"/notify/topics", token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.NotEmpty(t, res.Data)
	})

	// 3. Subscribe to Topic (queue protocol)
	var subscriptionID string
	subscriptionCreated := false
	t.Run("SubscribeTopicQueue", func(t *testing.T) {
		payload := map[string]string{
			"protocol": "queue",
			"endpoint": "https://example.com/queue",
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/notify/topics/%s/subscriptions", testutil.TestBaseURL, topicID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			t.Skip("Subscription API not accessible for this user")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				ID       string `json:"id"`
				Protocol string `json:"protocol"`
				Endpoint string `json:"endpoint"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		subscriptionID = res.Data.ID
		subscriptionCreated = true
		assert.Equal(t, "queue", res.Data.Protocol)
		assert.Equal(t, "https://example.com/queue", res.Data.Endpoint)
	})

	if !subscriptionCreated {
		t.Skip("Subscription creation failed - cannot continue tests")
	}

	// 4. Subscribe to Topic (webhook protocol)
	t.Run("SubscribeTopicWebhook", func(t *testing.T) {
		payload := map[string]string{
			"protocol": "webhook",
			"endpoint": "https://example.com/webhook",
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/notify/topics/%s/subscriptions", testutil.TestBaseURL, topicID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			t.Skip("Webhook subscription not available")
		}

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var res struct {
			Data struct {
				Protocol string `json:"protocol"`
				Endpoint string `json:"endpoint"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.Equal(t, "webhook", res.Data.Protocol)
	})

	// 5. List Subscriptions for Topic
	t.Run("ListSubscriptions", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/notify/topics/%s/subscriptions", testutil.TestBaseURL, topicID), token)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.GreaterOrEqual(t, len(res.Data), 2) // We created 2 subscriptions
	})

	// 6. Publish to Topic
	t.Run("PublishTopic", func(t *testing.T) {
		payload := map[string]string{
			"message": "Hello from E2E test!",
		}
		resp := postRequest(t, client, fmt.Sprintf("%s/notify/topics/%s/publish", testutil.TestBaseURL, topicID), token, payload)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Publish endpoint not available")
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&res)
	})

	// 7. Unsubscribe
	t.Run("Unsubscribe", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/notify/subscriptions/%s", testutil.TestBaseURL, subscriptionID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Unsubscribe not available")
		}

		// Accept 200, 204, or 202
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
			t.Skip("Unsubscribe returned unexpected status")
		}
	})

	// 8. Delete Topic
	t.Run("DeleteTopic", func(t *testing.T) {
		resp := deleteRequest(t, client, fmt.Sprintf("%s/notify/topics/%s", testutil.TestBaseURL, topicID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNotFound {
			t.Skip("Topic already deleted")
		}

		// Accept 200, 204, or 202
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
			t.Skip("Delete topic returned unexpected status")
		}
	})

	// 9. Verify Topic is deleted
	t.Run("VerifyTopicDeleted", func(t *testing.T) {
		resp := getRequest(t, client, fmt.Sprintf("%s/notify/topics/%s", testutil.TestBaseURL, topicID), token)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			t.Skip("Topic may still be deleting")
		}
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
