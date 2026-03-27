package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ApprovalOutcome is the result of a HITL approval wait.
type ApprovalOutcome string

const (
	OutcomeApproved ApprovalOutcome = "approved"
	OutcomeDenied   ApprovalOutcome = "denied"
	OutcomeTimeout  ApprovalOutcome = "timeout"
)

// PendingRequest is written to hitl/pending/<token>.json when HITL approval is needed.
type PendingRequest struct {
	Token     string    `json:"token"`
	AgentID   string    `json:"agent_id,omitempty"`
	ServerID  int64     `json:"server_id,omitempty"`
	ClusterID int64     `json:"cluster_id,omitempty"`
	Host      string    `json:"host,omitempty"`
	Commands  []string  `json:"commands"`
	Intent    string    `json:"intent,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// HITLDir returns the base directory for HITL files: <configDir>/hitl/
func HITLDir(configDir string) string {
	return filepath.Join(configDir, "hitl")
}

func pendingDir(configDir string) string  { return filepath.Join(HITLDir(configDir), "pending") }
func approvedDir(configDir string) string { return filepath.Join(HITLDir(configDir), "approved") }
func deniedDir(configDir string) string   { return filepath.Join(HITLDir(configDir), "denied") }

// RequestApproval writes a pending approval request, prints a human-readable
// notice to stderr, and polls until the request is approved, denied, or times out.
// On return it cleans up the pending and outcome files.
func RequestApproval(ctx context.Context, configDir string, req PendingRequest, timeout time.Duration) (ApprovalOutcome, error) {
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	req.ExpiresAt = req.CreatedAt.Add(timeout)

	for _, dir := range []string{pendingDir(configDir), approvedDir(configDir), deniedDir(configDir)} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return OutcomeTimeout, fmt.Errorf("hitl: mkdir %s: %w", dir, err)
		}
	}

	pendingFile := filepath.Join(pendingDir(configDir), req.Token+".json")
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return OutcomeTimeout, fmt.Errorf("hitl: marshal request: %w", err)
	}
	if err := os.WriteFile(pendingFile, data, 0600); err != nil {
		return OutcomeTimeout, fmt.Errorf("hitl: write pending: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n\u26a0 HITL approval required\n  token:   %s\n  run:     alogin agent approve %s\n  expires: %s\n\n",
		req.Token, req.Token, req.ExpiresAt.Format(time.RFC3339))

	approvedFile := filepath.Join(approvedDir(configDir), req.Token)
	deniedFile := filepath.Join(deniedDir(configDir), req.Token)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = cleanup(pendingFile, approvedFile, deniedFile)
			return OutcomeTimeout, nil
		case <-deadline.C:
			_ = cleanup(pendingFile, approvedFile, deniedFile)
			fmt.Fprintf(os.Stderr, "⏱ HITL timeout for token %s\n", req.Token)
			return OutcomeTimeout, nil
		case <-ticker.C:
			if fileExists(approvedFile) {
				_ = cleanup(pendingFile, approvedFile, deniedFile)
				return OutcomeApproved, nil
			}
			if fileExists(deniedFile) {
				_ = cleanup(pendingFile, approvedFile, deniedFile)
				return OutcomeDenied, nil
			}
		}
	}
}

// Approve writes the approval marker file for the given token.
// Called by "alogin agent approve <token>".
func Approve(configDir, token string) error {
	if err := os.MkdirAll(approvedDir(configDir), 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(approvedDir(configDir), token), []byte(time.Now().UTC().Format(time.RFC3339)), 0600)
}

// Deny writes the denial marker file for the given token.
// Called by "alogin agent deny <token>".
func Deny(configDir, token string) error {
	if err := os.MkdirAll(deniedDir(configDir), 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(deniedDir(configDir), token), []byte(time.Now().UTC().Format(time.RFC3339)), 0600)
}

// ListPending reads all pending JSON files and returns those that have not yet expired.
func ListPending(configDir string) ([]*PendingRequest, error) {
	dir := pendingDir(configDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	now := time.Now()
	var pending []*PendingRequest
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var req PendingRequest
		if err := json.Unmarshal(data, &req); err != nil {
			continue
		}
		if req.ExpiresAt.After(now) {
			pending = append(pending, &req)
		}
	}
	return pending, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func cleanup(files ...string) error {
	var last error
	for _, f := range files {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			last = err
		}
	}
	return last
}
