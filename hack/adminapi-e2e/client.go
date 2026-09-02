package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

const maxResponseBytes = 2 << 20

type responseBody struct {
	Code    int             `json:"code"`
	Reason  string          `json:"reason"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

type apiFailure struct {
	status  int
	code    int
	reason  string
	message string
}

type apiClient struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func (e *apiFailure) Error() string {
	return fmt.Sprintf("status=%d code=%d reason=%s message=%s", e.status, e.code, e.reason, e.message)
}

func newAPIClient(rawBaseURL string) (*apiClient, error) {
	baseURL, err := url.Parse(strings.TrimRight(rawBaseURL, "/"))
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" {
		return nil, fmt.Errorf("INGATE_E2E_BASE_URL must be an absolute HTTP URL")
	}
	if baseURL.User != nil {
		return nil, fmt.Errorf("INGATE_E2E_BASE_URL must not contain credentials")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	return &apiClient{baseURL: baseURL, httpClient: &http.Client{Jar: jar}}, nil
}

func (c *apiClient) login(ctx context.Context, username, password string) error {
	body := map[string]string{"username": username, "password": password}
	return c.call(ctx, http.MethodPost, "/auth/session", body, nil)
}

func (c *apiClient) call(ctx context.Context, method, path string, requestBody, responseData any) error {
	status, body, err := c.request(ctx, method, path, requestBody)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 || body.Code < 200 || body.Code >= 300 {
		return &apiFailure{status: status, code: body.Code, reason: body.Reason, message: body.Message}
	}
	if responseData == nil || len(body.Data) == 0 || bytes.Equal(body.Data, []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(body.Data, responseData); err != nil {
		return fmt.Errorf("decode response data: %w", err)
	}
	return nil
}

func (c *apiClient) expectFailure(
	ctx context.Context,
	method string,
	path string,
	requestBody any,
	wantStatus int,
	wantReason string,
) error {
	status, body, err := c.request(ctx, method, path, requestBody)
	if err != nil {
		return err
	}
	if status != wantStatus || body.Code != wantStatus || body.Reason != wantReason {
		return fmt.Errorf(
			"unexpected failure: status=%d code=%d reason=%s message=%s",
			status,
			body.Code,
			body.Reason,
			body.Message,
		)
	}
	return nil
}

func (c *apiClient) request(
	ctx context.Context,
	method string,
	path string,
	requestBody any,
) (int, responseBody, error) {
	var reader io.Reader
	if requestBody != nil {
		data, err := json.Marshal(requestBody)
		if err != nil {
			return 0, responseBody{}, fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL.String()+path, reader)
	if err != nil {
		return 0, responseBody{}, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, responseBody{}, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return 0, responseBody{}, fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return 0, responseBody{}, fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
	}
	var body responseBody
	if err := json.Unmarshal(data, &body); err != nil {
		return 0, responseBody{}, fmt.Errorf("decode response envelope: %w", err)
	}
	return response.StatusCode, body, nil
}
