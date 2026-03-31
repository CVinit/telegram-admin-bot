package dujiao

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginStoresReturnedTokenEnvelope(t *testing.T) {
	api := newTestServerReturningJSON(t, 200, map[string]any{
		"status_code": 0,
		"msg":         "success",
		"data": map[string]any{
			"token":      "jwt-demo",
			"expires_at": "2026-03-31T18:00:00Z",
			"user":       map[string]any{"id": 3, "username": "ops"},
		},
	})
	client := New(api.URL)
	resp, err := client.Login(context.Background(), "ops", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Token != "jwt-demo" {
		t.Fatalf("unexpected token: %s", resp.Token)
	}
}

func TestGetAuthzMeUsesBearerToken(t *testing.T) {
	const wantToken = "jwt-ops"
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+wantToken {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		writeJSON(t, w, http.StatusOK, map[string]any{
			"status_code": 0,
			"msg":         "success",
			"data": map[string]any{
				"admin_id": 3,
				"is_super": false,
				"roles":    []string{"role:ops"},
				"policies": []map[string]any{
					{"subject": "role:ops", "object": "/api/v1/admin/orders", "action": "GET"},
				},
			},
		})
	}))
	t.Cleanup(api.Close)

	client := New(api.URL)
	resp, err := client.GetAuthzMe(context.Background(), wantToken)
	if err != nil {
		t.Fatal(err)
	}
	if resp.AdminID != 3 {
		t.Fatalf("unexpected admin id: %d", resp.AdminID)
	}
	if len(resp.Policies) != 1 || resp.Policies[0].Object != "/api/v1/admin/orders" {
		t.Fatalf("unexpected policies: %#v", resp.Policies)
	}
}

func TestListOrdersDecodesPagination(t *testing.T) {
	api := newTestServerReturningJSON(t, http.StatusOK, map[string]any{
		"status_code": 0,
		"msg":         "success",
		"data": []map[string]any{
			{
				"id":           11,
				"order_no":     "DJ202603310001",
				"status":       "paid",
				"total_amount": "99.00",
			},
		},
		"pagination": map[string]any{
			"page":       1,
			"page_size":  20,
			"total":      1,
			"total_page": 1,
		},
	})

	client := New(api.URL)
	resp, err := client.ListOrders(context.Background(), "jwt-demo", ListOrdersParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("unexpected item count: %d", len(resp.Items))
	}
	if resp.Pagination.Total != 1 {
		t.Fatalf("unexpected pagination total: %d", resp.Pagination.Total)
	}
}

func TestGetSMTPSettingsDecodesData(t *testing.T) {
	api := newTestServerReturningJSON(t, http.StatusOK, map[string]any{
		"status_code": 0,
		"msg":         "success",
		"data": map[string]any{
			"enabled":      true,
			"host":         "smtp.example.com",
			"port":         587,
			"username":     "ops@example.com",
			"password":     "",
			"has_password": true,
			"from":         "ops@example.com",
			"from_name":    "Ops",
			"use_tls":      true,
			"use_ssl":      false,
			"verify_code": map[string]any{
				"expire_minutes":        10,
				"send_interval_seconds": 60,
				"max_attempts":          5,
				"length":                6,
			},
		},
	})

	client := New(api.URL)
	resp, err := client.GetSMTPSettings(context.Background(), "jwt-demo")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Host != "smtp.example.com" {
		t.Fatalf("unexpected host: %s", resp.Host)
	}
	if !resp.HasPassword {
		t.Fatal("expected has_password=true")
	}
}

func TestAPIErrorPreservesStatusMsgAndRequestID(t *testing.T) {
	api := newTestServerReturningJSON(t, http.StatusOK, map[string]any{
		"status_code": 40101,
		"msg":         "error.token_invalid",
		"data": map[string]any{
			"request_id": "req-abc",
		},
	})

	client := New(api.URL)
	_, err := client.GetAuthzMe(context.Background(), "bad-token")
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got: %T", err)
	}
	if apiErr.StatusCode != 40101 {
		t.Fatalf("unexpected status code: %d", apiErr.StatusCode)
	}
	if apiErr.Message != "error.token_invalid" {
		t.Fatalf("unexpected message: %s", apiErr.Message)
	}
	if apiErr.RequestID != "req-abc" {
		t.Fatalf("unexpected request id: %s", apiErr.RequestID)
	}
}

func newTestServerReturningJSON(t *testing.T, status int, payload map[string]any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, status, payload)
	}))
	t.Cleanup(server.Close)
	return server
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, payload map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode response payload: %v", err)
	}
}
