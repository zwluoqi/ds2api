package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/config"
)

func TestExtractCreateSessionIDSupportsLegacyShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"id": "legacy-session-id",
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "legacy-session-id" {
		t.Fatalf("expected legacy session id, got %q", got)
	}
}

func TestExtractCreateSessionIDSupportsNestedChatSessionShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"chat_session": map[string]any{
					"id":         "nested-session-id",
					"model_type": "default",
				},
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "nested-session-id" {
		t.Fatalf("expected nested session id, got %q", got)
	}
}

func TestLoginUsesConfiguredDeviceID(t *testing.T) {
	var payload map[string]any
	client := &Client{
		regular: doerFunc(func(req *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			return loginTokenResponse("login-token"), nil
		}),
		fallback: &http.Client{},
	}

	token, err := client.Login(context.Background(), config.Account{
		Email:    "user@example.com",
		Password: "pwd",
		DeviceID: " account-device-1 ",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if token != "login-token" {
		t.Fatalf("unexpected token: %q", token)
	}
	if got, _ := payload["device_id"].(string); got != "account-device-1" {
		t.Fatalf("expected configured device_id, got %q", got)
	}
}

func TestLoginFallsBackToDefaultDeviceID(t *testing.T) {
	var payload map[string]any
	client := &Client{
		regular: doerFunc(func(req *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			return loginTokenResponse("login-token"), nil
		}),
		fallback: &http.Client{},
	}

	if _, err := client.Login(context.Background(), config.Account{Email: "user@example.com", Password: "pwd"}); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if got, _ := payload["device_id"].(string); got != defaultLoginDeviceID {
		t.Fatalf("expected default device_id, got %q", got)
	}
}

func loginTokenResponse(token string) *http.Response {
	body := `{"code":0,"data":{"biz_code":0,"biz_data":{"user":{"token":"` + token + `"}}}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
