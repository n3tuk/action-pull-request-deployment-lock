package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLockSerialization(t *testing.T) {
	now := time.Now()
	lock := Lock{
		Project:   "test-project",
		Branch:    "main",
		CreatedAt: now,
		Owner:     "test-owner",
	}

	// Serialize
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatalf("Failed to marshal lock: %v", err)
	}

	// Deserialize
	var decoded Lock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal lock: %v", err)
	}

	// Compare
	if decoded.Project != lock.Project {
		t.Errorf("Project mismatch: got %s, want %s", decoded.Project, lock.Project)
	}
	if decoded.Branch != lock.Branch {
		t.Errorf("Branch mismatch: got %s, want %s", decoded.Branch, lock.Branch)
	}
	if decoded.Owner != lock.Owner {
		t.Errorf("Owner mismatch: got %s, want %s", decoded.Owner, lock.Owner)
	}
	// Time comparison with truncation due to JSON serialization precision
	if !decoded.CreatedAt.Truncate(time.Second).Equal(lock.CreatedAt.Truncate(time.Second)) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", decoded.CreatedAt, lock.CreatedAt)
	}
}

func TestLockRequestSerialization(t *testing.T) {
	req := LockRequest{
		Project: "test-project",
		Branch:  "feature-branch",
	}

	// Serialize
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Deserialize
	var decoded LockRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	// Compare
	if decoded.Project != req.Project {
		t.Errorf("Project mismatch: got %s, want %s", decoded.Project, req.Project)
	}
	if decoded.Branch != req.Branch {
		t.Errorf("Branch mismatch: got %s, want %s", decoded.Branch, req.Branch)
	}
}

func TestUnlockRequestSerialization(t *testing.T) {
	req := UnlockRequest{
		Project: "test-project",
		Branch:  "feature-branch",
	}

	// Serialize
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Deserialize
	var decoded UnlockRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	// Compare
	if decoded.Project != req.Project {
		t.Errorf("Project mismatch: got %s, want %s", decoded.Project, req.Project)
	}
	if decoded.Branch != req.Branch {
		t.Errorf("Branch mismatch: got %s, want %s", decoded.Branch, req.Branch)
	}
}

func TestLockResponseSerialization(t *testing.T) {
	lock := &Lock{
		Project:   "test-project",
		Branch:    "main",
		CreatedAt: time.Now(),
		Owner:     "test-owner",
	}

	resp := LockResponse{
		Status:  "locked",
		Message: "Lock acquired",
		Lock:    lock,
	}

	// Serialize
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Deserialize
	var decoded LockResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Compare
	if decoded.Status != resp.Status {
		t.Errorf("Status mismatch: got %s, want %s", decoded.Status, resp.Status)
	}
	if decoded.Message != resp.Message {
		t.Errorf("Message mismatch: got %s, want %s", decoded.Message, resp.Message)
	}
	if decoded.Lock == nil {
		t.Fatal("Lock is nil")
	}
	if decoded.Lock.Project != lock.Project {
		t.Errorf("Lock project mismatch: got %s, want %s", decoded.Lock.Project, lock.Project)
	}
}
