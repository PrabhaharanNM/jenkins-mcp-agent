package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// resultsDir is the directory where analysis results are stored.
var resultsDir = filepath.Join(os.TempDir(), "mcp-results")

// Save marshals the AnalysisResult to JSON and writes it to disk.
func Save(analysisID string, result *models.AnalysisResult) error {
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return fmt.Errorf("failed to create results directory: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal analysis result: %w", err)
	}

	path := filepath.Join(resultsDir, analysisID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write analysis result to %s: %w", path, err)
	}

	return nil
}

// SaveStatus writes a minimal JSON file containing only the status field.
// This allows the Java plugin to poll for status while analysis is in progress.
func SaveStatus(analysisID string, status string) error {
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return fmt.Errorf("failed to create results directory: %w", err)
	}

	statusObj := map[string]string{
		"status": status,
	}

	data, err := json.MarshalIndent(statusObj, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	path := filepath.Join(resultsDir, analysisID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write status to %s: %w", path, err)
	}

	return nil
}

// Load reads an AnalysisResult from disk by its analysis ID.
func Load(analysisID string) (*models.AnalysisResult, error) {
	path := filepath.Join(resultsDir, analysisID+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read analysis result from %s: %w", path, err)
	}

	var result models.AnalysisResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal analysis result: %w", err)
	}

	return &result, nil
}
