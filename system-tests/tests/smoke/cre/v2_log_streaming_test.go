package cre

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-testing-framework/framework"

	ttypes "github.com/smartcontractkit/chainlink/system-tests/tests/test-helpers/configuration"
)

// LokiQueryResponse represents the response from Loki's query_range API
type LokiQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

// ExecuteLogStreamingTest validates that logs with beholder_data_type are flowing to Loki
func ExecuteLogStreamingTest(t *testing.T, testEnv *ttypes.TestEnvironment) {
	testLogger := framework.L
	testLogger.Info().Msg("Starting Log Streaming Test")

	testLogger.Info().Msg("Waiting for logs to accumulate...")
	time.Sleep(10 * time.Second)

	lokiURL := "http://localhost:3030"
	logCount, err := queryLokiForBeholderLogs(lokiURL, 60)
	require.NoError(t, err, "Failed to query Loki")

	testLogger.Info().Int("logCount", logCount).Msg("Found logs with beholder_data_type")
	require.Greater(t, logCount, 0, "Expected to find logs with beholder_data_type=zap_log_message in Loki, but found none")

	testLogger.Info().Msg("✅ Log Streaming Test PASSED: beholder_data_type logs are flowing to Loki")
}

// queryLokiForBeholderLogs queries Loki for logs containing beholder_data_type in the last N seconds
func queryLokiForBeholderLogs(lokiBaseURL string, lastNSeconds int) (int, error) {
	end := time.Now()
	start := end.Add(-time.Duration(lastNSeconds) * time.Second)

	startNano := strconv.FormatInt(start.UnixNano(), 10)
	endNano := strconv.FormatInt(end.UnixNano(), 10)

	query := `{service_name="unknown_service:chainlink"} | json | beholder_data_type="zap_log_message"`

	queryURL := fmt.Sprintf("%s/loki/api/v1/query_range", lokiBaseURL)

	u, err := url.Parse(queryURL)
	if err != nil {
		return 0, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("query", query)
	q.Set("start", startNano)
	q.Set("end", endNano)
	q.Set("limit", "100")
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return 0, fmt.Errorf("failed to query Loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("Loki query failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %w", err)
	}

	var lokiResp LokiQueryResponse
	if err := json.Unmarshal(body, &lokiResp); err != nil {
		return 0, fmt.Errorf("failed to parse Loki response: %w", err)
	}

	totalLogs := 0
	for _, result := range lokiResp.Data.Result {
		totalLogs += len(result.Values)
	}

	return totalLogs, nil
}
