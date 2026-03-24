package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	origDir := resultsDir
	resultsDir = tmpDir
	defer func() { resultsDir = origDir }()

	result := &models.AnalysisResult{
		Status:           "completed",
		Category:         "CodeChange",
		RootCauseSummary: "Compilation error in App.java",
		ResponsibleTeam:  "Payments Team",
		TeamEmail:        "payments@example.com",
		JiraTicketKey:    "PROJ-1234",
	}

	// Save
	err := Save("test-001", result)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(tmpDir, "test-001.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("expected result file to exist")
	}

	// Load
	loaded, err := Load("test-001")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", loaded.Status)
	}
	if loaded.Category != "CodeChange" {
		t.Errorf("expected category 'CodeChange', got %q", loaded.Category)
	}
	if loaded.JiraTicketKey != "PROJ-1234" {
		t.Errorf("expected jira key 'PROJ-1234', got %q", loaded.JiraTicketKey)
	}
}

func TestSaveStatus(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := resultsDir
	resultsDir = tmpDir
	defer func() { resultsDir = origDir }()

	err := SaveStatus("test-002", "in-progress")
	if err != nil {
		t.Fatalf("SaveStatus failed: %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(filepath.Join(tmpDir, "test-002.json"))
	if err != nil {
		t.Fatalf("reading status file: %v", err)
	}

	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("parsing status JSON: %v", err)
	}
	if status.Status != "in-progress" {
		t.Errorf("expected 'in-progress', got %q", status.Status)
	}
}

func TestLoad_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := resultsDir
	resultsDir = tmpDir
	defer func() { resultsDir = origDir }()

	_, err := Load("does-not-exist")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}
