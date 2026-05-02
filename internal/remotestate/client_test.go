package remotestate_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sourceplane/orun/internal/remotestate"
)

func newTestClient(srv *httptest.Server) *remotestate.Client {
	return remotestate.NewClient(srv.URL, "test", remotestate.NewStaticTokenSource("test-token"))
}

func TestClient_AuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"runId":        "run-123",
			"status":       "pending",
			"planChecksum": "abc",
			"triggerType":  "ci",
			"createdAt":    "2024-01-01T00:00:00Z",
			"updatedAt":    "2024-01-01T00:00:00Z",
			"jobTotal":     0,
			"jobDone":      0,
			"jobFailed":    0,
			"dryRun":       false,
		})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetRun(context.Background(), "run-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("expected Bearer test-token, got %q", gotAuth)
	}
}

func TestClient_UserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"runId": "r", "status": "pending", "planChecksum": "x",
			"triggerType": "ci", "createdAt": "2024-01-01T00:00:00Z",
			"updatedAt": "2024-01-01T00:00:00Z",
		})
	}))
	defer srv.Close()

	c := remotestate.NewClient(srv.URL, "1.2.3", remotestate.NewStaticTokenSource("tok"))
	c.GetRun(context.Background(), "r")
	if !strings.HasPrefix(gotUA, "orun-cli/") {
		t.Errorf("expected User-Agent to start with orun-cli/, got %q", gotUA)
	}
}

func TestClient_ErrorEnvelopeParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "run not found",
			"code":  "NOT_FOUND",
		})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.GetRun(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "NOT_FOUND") {
		t.Errorf("expected NOT_FOUND in error, got: %v", err)
	}
}

func TestClient_AuthErrorReturnsHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "token invalid",
			"code":  "UNAUTHORIZED",
		})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	err := c.UpdateJob(context.Background(), "run", "job", "runner", "success", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected authentication hint in error, got: %v", err)
	}
}

func TestClient_CreateRun(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"runId": "new-run-id", "status": "pending",
			"planChecksum": "abc123", "triggerType": "ci",
			"createdAt": "2024-01-01T00:00:00Z",
			"updatedAt": "2024-01-01T00:00:00Z",
		})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	resp, err := c.CreateRun(context.Background(), remotestate.CreateRunRequest{
		Plan:   &remotestate.BackendPlan{Checksum: "abc123"},
		RunID:  "new-run-id",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RunID != "new-run-id" {
		t.Errorf("expected new-run-id, got %q", resp.RunID)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("could not decode request body: %v", err)
	}
	if body["runId"] != "new-run-id" {
		t.Errorf("expected runId in body, got %v", body["runId"])
	}
}

func TestClient_ClaimJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"claimed":     true,
			"takeover":    false,
			"depsWaiting": false,
		})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	resp, err := c.ClaimJob(context.Background(), "run-1", "job-1", "runner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Claimed {
		t.Error("expected claimed: true")
	}
}

func TestClient_ClaimJob_DepsWaiting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"claimed":     false,
			"depsWaiting": true,
		})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	resp, err := c.ClaimJob(context.Background(), "run-1", "job-1", "runner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Claimed {
		t.Error("expected claimed: false")
	}
	if !resp.DepsWaiting {
		t.Error("expected depsWaiting: true")
	}
}

func TestClient_UploadLog(t *testing.T) {
	var gotBody string
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "logRef": "ref-123"})
	}))
	defer srv.Close()

	c := newTestClient(srv)
	err := c.UploadLog(context.Background(), "run-1", "job-1", "some log output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %q", gotMethod)
	}
	if gotBody != "some log output" {
		t.Errorf("expected log content in body, got %q", gotBody)
	}
}

func TestClient_GetLog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("step output here"))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	log, err := c.GetLog(context.Background(), "run-1", "job-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log != "step output here" {
		t.Errorf("expected log content, got %q", log)
	}
}

func TestClient_GetLog_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	log, err := c.GetLog(context.Background(), "run-1", "missing-job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log != "" {
		t.Errorf("expected empty log for 404, got %q", log)
	}
}
