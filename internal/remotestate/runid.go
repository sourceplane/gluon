package remotestate

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
)

// DeriveRunID computes the remote run ID according to the documented precedence:
//
//  1. explicitID (from --exec-id or ORUN_EXEC_ID)
//  2. GitHub Actions: gh-{GITHUB_RUN_ID}-{GITHUB_RUN_ATTEMPT}-{planID}
//  3. Local fallback: local-{planID}-{randomHex6}
func DeriveRunID(planID, explicitID string) string {
	if explicitID != "" {
		return explicitID
	}

	if os.Getenv("GITHUB_ACTIONS") == "true" {
		ghaRunID := os.Getenv("GITHUB_RUN_ID")
		attempt := os.Getenv("GITHUB_RUN_ATTEMPT")
		if attempt == "" {
			attempt = "1"
		}
		if ghaRunID != "" {
			return fmt.Sprintf("gh-%s-%s-%s", ghaRunID, attempt, planID)
		}
	}

	// Local fallback with 3-byte random suffix.
	var b [3]byte
	_, _ = rand.Read(b[:])
	suffix := hex.EncodeToString(b[:])
	return fmt.Sprintf("local-%s-%s", planID, suffix)
}
