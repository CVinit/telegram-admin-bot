package dujiao

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginStoresReturnedTokenEnvelope(t *testing.T) {
	api := newJSONServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/admin/login" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
			t.Fatalf("unexpected content type: %q", got)
		}

		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Username != "ops" || body.Password != "secret" {
			t.Fatalf("unexpected login payload: %+v", body)
		}

		writeJSON(t, w, http.StatusOK, map[string]any{
			"status_code": 0,
			"msg":         "success",
			"data": map[string]any{
				"token":      "jwt-demo",
				"expires_at": "2026-03-31T18:00:00Z",
				"user":       map[string]any{"id": 3, "username": "ops"},
			},
		})
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
	api := newJSONServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/admin/authz/me" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
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
	})

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
	api := newJSONServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/admin/orders" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("page"); got != "1" {
			t.Fatalf("unexpected page query: %q", got)
		}
		if got := r.URL.Query().Get("page_size"); got != "20" {
			t.Fatalf("unexpected page_size query: %q", got)
		}
		if got := r.URL.Query().Get("status"); got != "paid" {
			t.Fatalf("unexpected status query: %q", got)
		}
		if got := r.URL.Query().Get("product_keyword"); got != "vip" {
			t.Fatalf("unexpected product_keyword query: %q", got)
		}

		writeJSON(t, w, http.StatusOK, map[string]any{
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
	})

	client := New(api.URL)
	resp, err := client.ListOrders(context.Background(), "jwt-demo", ListOrdersParams{
		Page:           1,
		PageSize:       20,
		Status:         "paid",
		ProductKeyword: "vip",
	})
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
	api := newJSONServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/admin/settings/smtp" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		writeJSON(t, w, http.StatusOK, map[string]any{
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

func TestAPIErrorUsesCodeFallbackAndPreservesRequestID(t *testing.T) {
	api := newJSONServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, map[string]any{
			"code": 40101,
			"msg":  "error.token_invalid",
			"data": map[string]any{
				"request_id": "req-abc",
			},
		})
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

func TestHTTPErrorFallsBackToHTTPStatusWhenEnvelopeCodeMissing(t *testing.T) {
	api := newJSONServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusUnauthorized, map[string]any{
			"msg": "error.token_invalid",
			"data": map[string]any{
				"request_id": "req-http",
			},
		})
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
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status code %d, got %d", http.StatusUnauthorized, apiErr.StatusCode)
	}
	if apiErr.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("expected http status %d, got %d", http.StatusUnauthorized, apiErr.HTTPStatus)
	}
	if apiErr.RequestID != "req-http" {
		t.Fatalf("unexpected request id: %s", apiErr.RequestID)
	}
}

func TestPlainTextHTTPErrorReturnsAPIError(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		if _, err := w.Write([]byte("upstream exploded")); err != nil {
			t.Fatalf("write response body: %v", err)
		}
	}))
	t.Cleanup(api.Close)

	client := New(api.URL)
	_, err := client.GetSMTPSettings(context.Background(), "jwt-demo")
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got: %T", err)
	}
	if apiErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected status code %d, got %d", http.StatusBadGateway, apiErr.StatusCode)
	}
	if apiErr.Message != "upstream exploded" {
		t.Fatalf("unexpected message: %q", apiErr.Message)
	}
}

func newJSONServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(handler))
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
