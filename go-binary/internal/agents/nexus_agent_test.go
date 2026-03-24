package agents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// newNexusTestServer creates an httptest.NewServer and returns both the server
// and a new AnalysisRequest whose Nexus.Url points at the test server.
// The caller must defer server.Close().
func newNexusTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *models.AnalysisRequest) {
	t.Helper()
	server := httptest.NewServer(handler)
	req := &models.AnalysisRequest{
		Nexus: models.NexusConfig{
			Url:      server.URL,
			Username: "admin",
			Password: "secret",
		},
	}
	return server, req
}

func TestNexusAgent_SkipWhenNoConfig(t *testing.T) {
	req := &models.AnalysisRequest{}
	agent := NewNexusAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{JobName: "my-job"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// With no config the agent should return a default (empty-ish) result.
	if len(result.MissingArtifacts) != 0 {
		t.Errorf("expected no missing artifacts, got %v", result.MissingArtifacts)
	}
}

func TestNexusAgent_ArtifactFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/service/rest/v1/search", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name != "my-app" {
			t.Errorf("expected search for 'my-app', got %q", name)
		}
		resp := nexusSearchResponse{
			Items: []nexusComponent{
				{ID: "1", Repository: "maven-releases", Name: "my-app", Version: "1.0.0"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server, req := newNexusTestServer(t, mux)
	defer server.Close()

	// Override sharedClient to point at the test server.
	origClient := sharedClient
	sharedClient = server.Client()
	defer func() { sharedClient = origClient }()

	agent := NewNexusAgent(req)
	result, err := agent.Analyze(context.Background(), &models.BuildContext{JobName: "folder/subfolder/my-app"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.ArtifactsAvailable {
		t.Error("expected ArtifactsAvailable=true")
	}
	if len(result.MissingArtifacts) != 0 {
		t.Errorf("expected no missing artifacts, got %v", result.MissingArtifacts)
	}
	if result.RepositoryStatus != "OK" {
		t.Errorf("expected RepositoryStatus 'OK', got %q", result.RepositoryStatus)
	}
}

func TestNexusAgent_ArtifactMissing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/service/rest/v1/search", func(w http.ResponseWriter, r *http.Request) {
		resp := nexusSearchResponse{Items: []nexusComponent{}}
		json.NewEncoder(w).Encode(resp)
	})

	server, req := newNexusTestServer(t, mux)
	defer server.Close()

	origClient := sharedClient
	sharedClient = server.Client()
	defer func() { sharedClient = origClient }()

	agent := NewNexusAgent(req)
	result, err := agent.Analyze(context.Background(), &models.BuildContext{JobName: "my-app"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ArtifactsAvailable {
		t.Error("expected ArtifactsAvailable=false")
	}
	if len(result.MissingArtifacts) != 1 {
		t.Fatalf("expected 1 missing artifact, got %d", len(result.MissingArtifacts))
	}
	if result.MissingArtifacts[0] != "my-app" {
		t.Errorf("expected missing artifact 'my-app', got %q", result.MissingArtifacts[0])
	}
	if result.RepositoryStatus != "ARTIFACT_MISSING" {
		t.Errorf("expected RepositoryStatus 'ARTIFACT_MISSING', got %q", result.RepositoryStatus)
	}
}

func TestNexusAgent_DeriveArtifactName(t *testing.T) {
	agent := &NexusAgent{}

	tests := []struct {
		name     string
		jobName  string
		expected string
	}{
		{name: "multi-segment path", jobName: "folder/subfolder/my-app", expected: "my-app"},
		{name: "single segment", jobName: "single", expected: "single"},
		{name: "empty string", jobName: "", expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := agent.deriveArtifactName(tc.jobName)
			if got != tc.expected {
				t.Errorf("deriveArtifactName(%q) = %q, want %q", tc.jobName, got, tc.expected)
			}
		})
	}
}

func TestNexusAgent_SearchFailed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/service/rest/v1/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	})

	server, req := newNexusTestServer(t, mux)
	defer server.Close()

	origClient := sharedClient
	sharedClient = server.Client()
	defer func() { sharedClient = origClient }()

	agent := NewNexusAgent(req)
	result, err := agent.Analyze(context.Background(), &models.BuildContext{JobName: "my-app"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RepositoryStatus != "SEARCH_FAILED" {
		t.Errorf("expected RepositoryStatus 'SEARCH_FAILED', got %q", result.RepositoryStatus)
	}
}
