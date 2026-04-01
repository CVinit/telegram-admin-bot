package dujiao

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultHTTPTimeout = 15 * time.Second
const maxResponseBodyBytes = 2 << 20

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type APIError struct {
	StatusCode int
	Message    string
	RequestID  string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.HTTPStatus > 0 && e.StatusCode == 0 {
		return fmt.Sprintf("dujiao api error: http_status=%d msg=%q", e.HTTPStatus, e.Message)
	}
	if e.RequestID != "" {
		return fmt.Sprintf("dujiao api error: status_code=%d http_status=%d msg=%q request_id=%s", e.StatusCode, e.HTTPStatus, e.Message, e.RequestID)
	}
	return fmt.Sprintf("dujiao api error: status_code=%d http_status=%d msg=%q", e.StatusCode, e.HTTPStatus, e.Message)
}

type responseEnvelopeRaw struct {
	StatusCode int             `json:"status_code"`
	Code       int             `json:"code"`
	Msg        string          `json:"msg"`
	Data       json.RawMessage `json:"data"`
}

type pageEnvelopeRaw struct {
	StatusCode int             `json:"status_code"`
	Code       int             `json:"code"`
	Msg        string          `json:"msg"`
	Data       json.RawMessage `json:"data"`
	Pagination json.RawMessage `json:"pagination"`
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

func (c *Client) doJSON(ctx context.Context, method, path string, token string, in any, out any) error {
	envelope, err := c.doRequest(ctx, method, path, token, in)
	if err != nil {
		return err
	}
	if out == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode response data: %w", err)
	}
	return nil
}

func (c *Client) doJSONPage(ctx context.Context, method, path string, token string, in any, out any, pagination *Pagination) error {
	envelope, err := c.doPageRequest(ctx, method, path, token, in)
	if err != nil {
		return err
	}
	if out != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("decode paged response data: %w", err)
		}
	}
	if pagination != nil && len(envelope.Pagination) > 0 && string(envelope.Pagination) != "null" {
		if err := json.Unmarshal(envelope.Pagination, pagination); err != nil {
			return fmt.Errorf("decode pagination: %w", err)
		}
	}
	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, token string, in any) (*responseEnvelopeRaw, error) {
	if c == nil {
		return nil, errors.New("dujiao client is nil")
	}
	if c.baseURL == "" {
		return nil, errors.New("dujiao base URL is empty")
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	var body io.Reader
	if in != nil {
		payload, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("marshal request payload: %w", err)
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+normalizePath(path), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token = strings.TrimSpace(token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if len(rawBody) == 0 {
		if resp.StatusCode >= http.StatusBadRequest {
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Message:    http.StatusText(resp.StatusCode),
				HTTPStatus: resp.StatusCode,
			}
		}
		return &responseEnvelopeRaw{}, nil
	}

	var envelope responseEnvelopeRaw
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		if resp.StatusCode >= http.StatusBadRequest {
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Message:    strings.TrimSpace(string(rawBody)),
				HTTPStatus: resp.StatusCode,
			}
		}
		return nil, fmt.Errorf("decode response envelope: %w", err)
	}

	statusCode := envelope.StatusCode
	if statusCode == 0 && envelope.Code != 0 {
		statusCode = envelope.Code
	}
	if statusCode == 0 && resp.StatusCode >= http.StatusBadRequest {
		statusCode = resp.StatusCode
	}
	if statusCode != 0 || resp.StatusCode >= http.StatusBadRequest {
		msg := strings.TrimSpace(envelope.Msg)
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return nil, &APIError{
			StatusCode: statusCode,
			Message:    msg,
			RequestID:  extractRequestID(envelope.Data),
			HTTPStatus: resp.StatusCode,
		}
	}

	return &envelope, nil
}

func (c *Client) doPageRequest(ctx context.Context, method, path string, token string, in any) (*pageEnvelopeRaw, error) {
	if c == nil {
		return nil, errors.New("dujiao client is nil")
	}
	if c.baseURL == "" {
		return nil, errors.New("dujiao base URL is empty")
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	var body io.Reader
	if in != nil {
		payload, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("marshal request payload: %w", err)
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+normalizePath(path), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token = strings.TrimSpace(token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if len(rawBody) == 0 {
		if resp.StatusCode >= http.StatusBadRequest {
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Message:    http.StatusText(resp.StatusCode),
				HTTPStatus: resp.StatusCode,
			}
		}
		return &pageEnvelopeRaw{}, nil
	}

	var envelope pageEnvelopeRaw
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		if resp.StatusCode >= http.StatusBadRequest {
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Message:    strings.TrimSpace(string(rawBody)),
				HTTPStatus: resp.StatusCode,
			}
		}
		return nil, fmt.Errorf("decode paged response envelope: %w", err)
	}

	statusCode := envelope.StatusCode
	if statusCode == 0 && envelope.Code != 0 {
		statusCode = envelope.Code
	}
	if statusCode == 0 && resp.StatusCode >= http.StatusBadRequest {
		statusCode = resp.StatusCode
	}
	if statusCode != 0 || resp.StatusCode >= http.StatusBadRequest {
		msg := strings.TrimSpace(envelope.Msg)
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return nil, &APIError{
			StatusCode: statusCode,
			Message:    msg,
			RequestID:  extractRequestID(envelope.Data),
			HTTPStatus: resp.StatusCode,
		}
	}

	return &envelope, nil
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func extractRequestID(data json.RawMessage) string {
	if len(data) == 0 || string(data) == "null" {
		return ""
	}

	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return ""
	}

	if value, ok := object["request_id"].(string); ok {
		return strings.TrimSpace(value)
	}

	nested, ok := object["data"].(map[string]any)
	if !ok {
		return ""
	}
	value, ok := nested["request_id"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
