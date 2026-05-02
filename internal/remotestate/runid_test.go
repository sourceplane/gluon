package remotestate_test

import (
	"os"
	"strings"
	"testing"

	"github.com/sourceplane/orun/internal/remotestate"
)

func TestDeriveRunID_Explicit(t *testing.T) {
	id := remotestate.DeriveRunID("abc123", "my-explicit-id")
	if id != "my-explicit-id" {
		t.Errorf("expected my-explicit-id, got %q", id)
	}
}

func TestDeriveRunID_GitHubActions(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_RUN_ID", "99887766")
	t.Setenv("GITHUB_RUN_ATTEMPT", "2")

	id := remotestate.DeriveRunID("planABC", "")
	expected := "gh-99887766-2-planABC"
	if id != expected {
		t.Errorf("expected %q, got %q", expected, id)
	}
}

func TestDeriveRunID_GitHubActionsDefaultAttempt(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_RUN_ID", "12345")
	os.Unsetenv("GITHUB_RUN_ATTEMPT")

	id := remotestate.DeriveRunID("planXYZ", "")
	expected := "gh-12345-1-planXYZ"
	if id != expected {
		t.Errorf("expected %q, got %q", expected, id)
	}
}

func TestDeriveRunID_LocalFallback(t *testing.T) {
	os.Unsetenv("GITHUB_ACTIONS")
	os.Unsetenv("GITHUB_RUN_ID")

	id := remotestate.DeriveRunID("abc123", "")
	if !strings.HasPrefix(id, "local-abc123-") {
		t.Errorf("expected local-abc123-<hex>, got %q", id)
	}
}

func TestDeriveRunID_LocalFallback_IncludesPlanID(t *testing.T) {
	os.Unsetenv("GITHUB_ACTIONS")

	planID := "aabbcc"
	id := remotestate.DeriveRunID(planID, "")
	if !strings.Contains(id, planID) {
		t.Errorf("local run ID should include planID %q, got %q", planID, id)
	}
}
