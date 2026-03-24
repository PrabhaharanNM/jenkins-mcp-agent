package agents

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

func TestGithubAgent_SkipWhenNoToken(t *testing.T) {
	req := &models.AnalysisRequest{
		GitHub: models.GitHubSCMConfig{Token: ""},
	}
	agent := NewGithubAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.RecentCommits) != 0 {
		t.Errorf("expected empty RecentCommits, got %d", len(result.RecentCommits))
	}
}

func TestGithubAgent_FetchCommits(t *testing.T) {
	commits := []ghCommitResponse{
		{
			SHA: "abc123",
			Commit: ghCommitData{
				Message: "fix: resolve null pointer",
				Author:  ghAuthorData{Name: "Developer One", Date: "2025-01-15T10:00:00Z"},
			},
			Author: ghUser{Login: "dev1"},
		},
		{
			SHA: "def456",
			Commit: ghCommitData{
				Message: "feat: add logging",
				Author:  ghAuthorData{Name: "Developer Two", Date: "2025-01-14T09:00:00Z"},
			},
			Author: ghUser{Login: "dev2"},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/myorg/myrepo/commits", func(w http.ResponseWriter, r *http.Request) {
		// If no SHA path suffix, this is the list endpoint.
		if r.URL.Query().Get("per_page") != "" {
			json.NewEncoder(w).Encode(commits)
			return
		}
		// Commit detail endpoint for changed files.
		detail := ghCommitDetail{
			Files: []ghFile{{Filename: "main.go"}, {Filename: "utils.go"}},
		}
		json.NewEncoder(w).Encode(detail)
	})
	// Separate handler for commit detail (with SHA in path).
	mux.HandleFunc("/repos/myorg/myrepo/commits/abc123", func(w http.ResponseWriter, r *http.Request) {
		detail := ghCommitDetail{
			Files: []ghFile{{Filename: "main.go"}, {Filename: "utils.go"}},
		}
		json.NewEncoder(w).Encode(detail)
	})
	mux.HandleFunc("/repos/myorg/myrepo/contents/CODEOWNERS", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/repos/myorg/myrepo/contents/.github/CODEOWNERS", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	origClient := sharedClient
	sharedClient = server.Client()
	defer func() { sharedClient = origClient }()

	req := &models.AnalysisRequest{
		GitHub: models.GitHubSCMConfig{
			Token:  "test-token",
			ApiUrl: server.URL,
			Owner:  "myorg",
			Repo:   "myrepo",
		},
	}
	agent := NewGithubAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.RecentCommits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(result.RecentCommits))
	}
	if result.RecentCommits[0].Hash != "abc123" {
		t.Errorf("expected first commit hash 'abc123', got %q", result.RecentCommits[0].Hash)
	}
	if result.RecentCommits[0].Author != "Developer One" {
		t.Errorf("expected first commit author 'Developer One', got %q", result.RecentCommits[0].Author)
	}
	if result.RecentCommits[1].Hash != "def456" {
		t.Errorf("expected second commit hash 'def456', got %q", result.RecentCommits[1].Hash)
	}
}

func TestGithubAgent_FetchChangedFiles(t *testing.T) {
	commits := []ghCommitResponse{
		{
			SHA: "sha1",
			Commit: ghCommitData{
				Message: "update",
				Author:  ghAuthorData{Name: "Dev", Date: "2025-01-15T10:00:00Z"},
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/commits", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("per_page") != "" {
			json.NewEncoder(w).Encode(commits)
			return
		}
	})
	mux.HandleFunc("/repos/org/repo/commits/sha1", func(w http.ResponseWriter, r *http.Request) {
		detail := ghCommitDetail{
			Files: []ghFile{
				{Filename: "src/main.go"},
				{Filename: "src/handler.go"},
				{Filename: "README.md"},
			},
		}
		json.NewEncoder(w).Encode(detail)
	})
	mux.HandleFunc("/repos/org/repo/contents/CODEOWNERS", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/repos/org/repo/contents/.github/CODEOWNERS", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	origClient := sharedClient
	sharedClient = server.Client()
	defer func() { sharedClient = origClient }()

	req := &models.AnalysisRequest{
		GitHub: models.GitHubSCMConfig{
			Token:  "tok",
			ApiUrl: server.URL,
			Owner:  "org",
			Repo:   "repo",
		},
	}
	agent := NewGithubAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ChangedFiles) != 3 {
		t.Fatalf("expected 3 changed files, got %d", len(result.ChangedFiles))
	}
	expected := []string{"src/main.go", "src/handler.go", "README.md"}
	for i, f := range expected {
		if result.ChangedFiles[i] != f {
			t.Errorf("changed file[%d] = %q, want %q", i, result.ChangedFiles[i], f)
		}
	}
}

func TestGithubAgent_FetchCodeOwners(t *testing.T) {
	codeownersContent := "* @team-leads\n/src/ @backend-team\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(codeownersContent))

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/commits", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ghCommitResponse{})
	})
	mux.HandleFunc("/repos/org/repo/contents/CODEOWNERS", func(w http.ResponseWriter, r *http.Request) {
		resp := ghFileContent{Content: encoded, Encoding: "base64"}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	origClient := sharedClient
	sharedClient = server.Client()
	defer func() { sharedClient = origClient }()

	req := &models.AnalysisRequest{
		GitHub: models.GitHubSCMConfig{
			Token:  "tok",
			ApiUrl: server.URL,
			Owner:  "org",
			Repo:   "repo",
		},
	}
	agent := NewGithubAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.CodeOwners != codeownersContent {
		t.Errorf("expected CodeOwners %q, got %q", codeownersContent, result.CodeOwners)
	}
}

func TestGithubAgent_CodeOwnersFallback(t *testing.T) {
	codeownersContent := "/docs/ @docs-team\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(codeownersContent))

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/commits", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ghCommitResponse{})
	})
	mux.HandleFunc("/repos/org/repo/contents/CODEOWNERS", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	})
	mux.HandleFunc("/repos/org/repo/contents/.github/CODEOWNERS", func(w http.ResponseWriter, r *http.Request) {
		resp := ghFileContent{Content: encoded, Encoding: "base64"}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	origClient := sharedClient
	sharedClient = server.Client()
	defer func() { sharedClient = origClient }()

	req := &models.AnalysisRequest{
		GitHub: models.GitHubSCMConfig{
			Token:  "tok",
			ApiUrl: server.URL,
			Owner:  "org",
			Repo:   "repo",
		},
	}
	agent := NewGithubAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.CodeOwners != codeownersContent {
		t.Errorf("expected CodeOwners %q, got %q", codeownersContent, result.CodeOwners)
	}
}

func TestGithubAgent_AuthHeaderVerification(t *testing.T) {
	var capturedAuth string

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/commits", func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode([]ghCommitResponse{})
	})
	mux.HandleFunc("/repos/org/repo/contents/CODEOWNERS", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/repos/org/repo/contents/.github/CODEOWNERS", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	origClient := sharedClient
	sharedClient = server.Client()
	defer func() { sharedClient = origClient }()

	req := &models.AnalysisRequest{
		GitHub: models.GitHubSCMConfig{
			Token:  "my-secret-token",
			ApiUrl: server.URL,
			Owner:  "org",
			Repo:   "repo",
		},
	}
	agent := NewGithubAgent(req)

	_, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Bearer my-secret-token"
	if capturedAuth != expected {
		t.Errorf("expected Authorization header %q, got %q", expected, capturedAuth)
	}
}
