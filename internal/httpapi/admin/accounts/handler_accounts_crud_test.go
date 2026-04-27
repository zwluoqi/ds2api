package accounts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/accountstats"
)

func TestListAccountsPageSizeCapIs5000(t *testing.T) {
	accounts := make([]string, 0, 150)
	for i := range 150 {
		accounts = append(accounts, fmt.Sprintf(`{"email":"u%d@example.com","password":"pwd"}`, i))
	}
	raw := fmt.Sprintf(`{"accounts":[%s]}`, strings.Join(accounts, ","))
	router := newHTTPAdminHarness(t, raw, &testingDSMock{})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, adminReq(http.MethodGet, "/accounts?page=1&page_size=200", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items, _ := payload["items"].([]any)
	if len(items) != 150 {
		t.Fatalf("expected all 150 accounts with page_size=200, got %d", len(items))
	}
	if ps, _ := payload["page_size"].(float64); ps != 200 {
		t.Fatalf("expected page_size=200 in response, got %v", payload["page_size"])
	}
}

func TestListAccountsPageSizeAbove5000ClampedTo5000(t *testing.T) {
	router := newHTTPAdminHarness(t, `{"accounts":[{"email":"u@example.com","password":"pwd"}]}`, &testingDSMock{})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, adminReq(http.MethodGet, "/accounts?page=1&page_size=9999", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ps, _ := payload["page_size"].(float64); ps != 5000 {
		t.Fatalf("expected page_size clamped to 5000, got %v", payload["page_size"])
	}
}

func TestUpdateAccountMetadataPreservesCredentials(t *testing.T) {
	h := newAdminTestHandler(t, `{
		"accounts":[{"email":"u@example.com","name":"old name","remark":"old remark","password":"secret","device_id":"old-device"}]
	}`)

	r := chi.NewRouter()
	r.Put("/admin/accounts/{identifier}", h.updateAccount)

	body := []byte(`{"name":"new name","remark":"new remark","device_id":"old-device"}`)
	req := httptest.NewRequest(http.MethodPut, "/admin/accounts/u@example.com", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	snap := h.Store.Snapshot()
	if len(snap.Accounts) != 1 {
		t.Fatalf("unexpected accounts after update: %#v", snap.Accounts)
	}
	acc := snap.Accounts[0]
	if acc.Email != "u@example.com" {
		t.Fatalf("identifier changed unexpectedly: %#v", acc)
	}
	if acc.Name != "new name" || acc.Remark != "new remark" {
		t.Fatalf("metadata update did not persist: %#v", acc)
	}
	if acc.Password != "secret" {
		t.Fatalf("password should be preserved, got %#v", acc)
	}
	if acc.DeviceID != "old-device" {
		t.Fatalf("device id should be preserved, got %#v", acc)
	}
}

func TestUpdateAccountDeviceIDClearsRuntimeToken(t *testing.T) {
	h := newAdminTestHandler(t, `{
		"accounts":[{"email":"u@example.com","password":"secret","device_id":"old-device"}]
	}`)
	if err := h.Store.UpdateAccountToken("u@example.com", "runtime-token"); err != nil {
		t.Fatalf("seed runtime token: %v", err)
	}

	r := chi.NewRouter()
	r.Put("/admin/accounts/{identifier}", h.updateAccount)

	req := httptest.NewRequest(http.MethodPut, "/admin/accounts/u@example.com", strings.NewReader(`{"device_id":"new-device"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	acc := h.Store.Snapshot().Accounts[0]
	if acc.DeviceID != "new-device" {
		t.Fatalf("device id update did not persist: %#v", acc)
	}
	if acc.Token != "" {
		t.Fatalf("expected token to be cleared after device id change, got %q", acc.Token)
	}
}

func TestListAccountsMasksTokenPreview(t *testing.T) {
	h := newAdminTestHandler(t, `{
		"accounts":[{"email":"u@example.com","password":"pwd"}]
	}`)
	if err := h.Store.UpdateAccountToken("u@example.com", "abcdefgh"); err != nil {
		t.Fatalf("seed runtime token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/accounts?page=1&page_size=10", nil)
	rec := httptest.NewRecorder()
	h.listAccounts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	items, _ := payload["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	first, _ := items[0].(map[string]any)
	if got, _ := first["token_preview"].(string); got != "ab****gh" {
		t.Fatalf("expected masked token preview, got %q", got)
	}
}

func TestListAccountsIncludesStats(t *testing.T) {
	h := newAdminTestHandler(t, `{
		"accounts":[{"email":"u@example.com","password":"pwd"}]
	}`)
	h.Stats = accountstats.New(t.TempDir())
	if err := h.Stats.Record("u@example.com", "deepseek-v4-flash"); err != nil {
		t.Fatalf("seed stats: %v", err)
	}
	if err := h.Stats.Record("u@example.com", "deepseek-v4-pro"); err != nil {
		t.Fatalf("seed stats: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/accounts?page=1&page_size=10", nil)
	rec := httptest.NewRecorder()
	h.listAccounts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	items, _ := payload["items"].([]any)
	first, _ := items[0].(map[string]any)
	stats, _ := first["stats"].(map[string]any)
	if got, _ := stats["daily_requests"].(float64); got != 2 {
		t.Fatalf("expected daily requests=2, got %#v", stats)
	}
	if got, _ := stats["total_flash_requests"].(float64); got != 1 {
		t.Fatalf("expected total flash requests=1, got %#v", stats)
	}
	if got, _ := stats["total_pro_requests"].(float64); got != 1 {
		t.Fatalf("expected total pro requests=1, got %#v", stats)
	}
}
