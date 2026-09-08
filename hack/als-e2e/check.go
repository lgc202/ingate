package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

const (
	probeInterval = 250 * time.Millisecond
	probeTimeout  = 90 * time.Second
)

var traceIDPattern = regexp.MustCompile(`"trace_id":"([0-9a-f]{32})"`)

type readiness struct {
	Reason         string `json:"reason"`
	WriteTarget    string `json:"write_target"`
	PendingRecords int64  `json:"pending_records"`
}

type queryResponse struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

type queryData struct {
	Result []json.RawMessage `json:"result"`
}

type lokiData struct {
	Result []struct {
		Values [][]string `json:"values"`
	} `json:"result"`
}

func waitHTTP(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("http", flag.ContinueOnError)
	address := flags.String("url", "", "HTTP URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *address == "" {
		return errors.New("url is required")
	}

	err := waitFor(ctx, func(ctx context.Context) (bool, error) {
		response, err := get(ctx, *address)
		if err != nil {
			return false, nil
		}
		defer func() { _ = response.Body.Close() }()
		_, _ = io.Copy(io.Discard, response.Body)
		return response.StatusCode >= 200 && response.StatusCode < 300, nil
	})
	if err != nil {
		return fmt.Errorf("wait for %s: %w", *address, err)
	}
	return nil
}

func waitReady(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("ready", flag.ContinueOnError)
	address := flags.String("url", "http://als:18092/readyz", "ALS readiness URL")
	target := flags.String("target", "", "expected write target")
	reason := flags.String("reason", "", "expected unready reason")
	pending := flags.Int64("pending", -1, "expected pending records")
	minimumPending := flags.Int64("min-pending", -1, "minimum pending records")
	if err := flags.Parse(args); err != nil {
		return err
	}

	err := waitFor(ctx, func(ctx context.Context) (bool, error) {
		response, err := get(ctx, *address)
		if err != nil {
			return false, nil
		}
		defer func() { _ = response.Body.Close() }()

		var status readiness
		if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
			return false, nil
		}
		if *target != "" && status.WriteTarget != *target {
			return false, nil
		}
		if *reason != "" && status.Reason != *reason {
			return false, nil
		}
		if *pending >= 0 && status.PendingRecords != *pending {
			return false, nil
		}
		if *minimumPending >= 0 && status.PendingRecords < *minimumPending {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("wait for ALS readiness at %s: %w", *address, err)
	}
	return nil
}

func verifyObservability(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("observe", flag.ContinueOnError)
	prometheusURL := flags.String("prometheus", "http://prometheus:9090", "Prometheus URL")
	lokiURL := flags.String("loki", "http://loki:3100", "Loki URL")
	tempoURL := flags.String("tempo", "http://tempo:3200", "Tempo URL")
	marker := flags.String("marker", "", "current run marker")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *marker == "" {
		return errors.New("marker is required")
	}

	if err := waitPrometheus(ctx, *prometheusURL, `up{job="ingate-als"} == 1`); err != nil {
		return fmt.Errorf("query ALS metrics: %w", err)
	}
	// Kafka 不可用和 WAL 满也会产生告警；只有精确命中丢弃告警，
	// 才能证明本次无效记录确实进入指标和规则评估链路。
	if err := waitPrometheus(
		ctx,
		*prometheusURL,
		`ALERTS{alertname="ALSRecordsDiscarded",alertstate=~"pending|firing"}`,
	); err != nil {
		return fmt.Errorf("query active ALS alert: %w", err)
	}

	traceID, err := waitLoki(ctx, *lokiURL, *marker)
	if err != nil {
		return err
	}
	if err := waitTempo(ctx, *tempoURL, traceID); err != nil {
		return err
	}

	fmt.Println(traceID)
	return nil
}

func waitPrometheus(ctx context.Context, baseURL, query string) error {
	return waitFor(ctx, func(ctx context.Context) (bool, error) {
		body, err := queryAPI(ctx, baseURL+"/api/v1/query?query="+url.QueryEscape(query))
		if err != nil {
			return false, nil
		}
		var response queryResponse
		if err := json.Unmarshal(body, &response); err != nil || response.Status != "success" {
			return false, nil
		}
		var data queryData
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return false, nil
		}
		return len(data.Result) > 0, nil
	})
}

func waitLoki(ctx context.Context, baseURL, marker string) (string, error) {
	var traceID string
	query := `{service_name="ingate-als"} |= ` + fmt.Sprintf("%q", marker)
	start := time.Now().Add(-10 * time.Minute).UnixNano()
	err := waitFor(ctx, func(ctx context.Context) (bool, error) {
		endpoint := fmt.Sprintf(
			"%s/loki/api/v1/query_range?query=%s&start=%d&limit=100",
			baseURL,
			url.QueryEscape(query),
			start,
		)
		body, err := queryAPI(ctx, endpoint)
		if err != nil {
			return false, nil
		}
		var response queryResponse
		if err := json.Unmarshal(body, &response); err != nil || response.Status != "success" {
			return false, nil
		}
		var data lokiData
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return false, nil
		}
		for _, stream := range data.Result {
			for _, value := range stream.Values {
				if len(value) != 2 {
					continue
				}
				match := traceIDPattern.FindStringSubmatch(value[1])
				if len(match) == 2 {
					traceID = match[1]
					return true, nil
				}
			}
		}
		return false, nil
	})
	if err != nil {
		return "", fmt.Errorf("query ALS log in Loki: %w", err)
	}
	return traceID, nil
}

func waitTempo(ctx context.Context, baseURL, traceID string) error {
	err := waitFor(ctx, func(ctx context.Context) (bool, error) {
		response, err := get(ctx, baseURL+"/api/traces/"+traceID)
		if err != nil {
			return false, nil
		}
		defer func() { _ = response.Body.Close() }()
		_, _ = io.Copy(io.Discard, response.Body)
		return response.StatusCode == http.StatusOK, nil
	})
	if err != nil {
		return fmt.Errorf("query ALS trace %s in Tempo: %w", traceID, err)
	}
	return nil
}

func queryAPI(ctx context.Context, address string) ([]byte, error) {
	response, err := get(ctx, address)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %s", address, response.Status)
	}
	return io.ReadAll(io.LimitReader(response.Body, 4<<20))
}

func get(ctx context.Context, address string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(request)
}

func waitFor(parent context.Context, condition func(context.Context) (bool, error)) error {
	ctx, cancel := context.WithTimeout(parent, probeTimeout)
	defer cancel()

	ticker := time.NewTicker(probeInterval)
	defer ticker.Stop()
	for {
		ready, err := condition(ctx)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
