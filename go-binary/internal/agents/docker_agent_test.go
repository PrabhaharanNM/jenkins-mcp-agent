package agents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

func TestDockerAgent_SkipWhenNoConfig(t *testing.T) {
	t.Helper()
	req := &models.AnalysisRequest{}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.ContainerStatuses) != 0 {
		t.Errorf("expected empty ContainerStatuses, got %d", len(result.ContainerStatuses))
	}
}

func TestDockerAgent_ListContainers(t *testing.T) {
	t.Helper()

	containers := []dockerContainer{
		{ID: "abc123def456", Names: []string{"/web-app"}, Image: "nginx:latest", State: "running", Status: "Up 2 hours"},
		{ID: "def456abc789", Names: []string{"/worker"}, Image: "python:3.9", State: "exited", Status: "Exited (1) 5 minutes ago"},
	}

	inspectResp := dockerInspect{
		State: dockerState{ExitCode: 1, OOMKilled: false, Error: ""},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(containers)
	})
	mux.HandleFunc("/v1.43/containers/def456abc789/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(inspectResp)
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(dockerDiskUsage{})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ContainerStatuses) != 2 {
		t.Fatalf("expected 2 container statuses, got %d", len(result.ContainerStatuses))
	}

	if result.ContainerStatuses[0].Name != "web-app" {
		t.Errorf("expected first container name 'web-app', got %q", result.ContainerStatuses[0].Name)
	}
	if result.ContainerStatuses[0].State != "running" {
		t.Errorf("expected first container state 'running', got %q", result.ContainerStatuses[0].State)
	}
	if result.ContainerStatuses[1].Name != "worker" {
		t.Errorf("expected second container name 'worker', got %q", result.ContainerStatuses[1].Name)
	}
	if result.ContainerStatuses[1].State != "exited" {
		t.Errorf("expected second container state 'exited', got %q", result.ContainerStatuses[1].State)
	}
}

func TestDockerAgent_OOMKilledDetection(t *testing.T) {
	t.Helper()

	containers := []dockerContainer{
		{ID: "oom123container", Names: []string{"/oom-victim"}, Image: "java:11", State: "exited", Status: "Exited (137)"},
	}

	inspectResp := dockerInspect{
		State: dockerState{ExitCode: 137, OOMKilled: true, Error: ""},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(containers)
	})
	mux.HandleFunc("/v1.43/containers/oom123container/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(inspectResp)
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(dockerDiskUsage{})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.OOMKilled) != 1 {
		t.Fatalf("expected 1 OOMKilled entry, got %d", len(result.OOMKilled))
	}
	if result.OOMKilled[0] != "oom-victim" {
		t.Errorf("expected OOMKilled entry 'oom-victim', got %q", result.OOMKilled[0])
	}
}

func TestDockerAgent_FailedContainers(t *testing.T) {
	t.Helper()

	containers := []dockerContainer{
		{ID: "dead123abc456", Names: []string{"/failed-svc"}, Image: "myapp:v1", State: "exited", Status: "Exited (1)"},
	}

	inspectResp := dockerInspect{
		State: dockerState{ExitCode: 1, OOMKilled: false, Error: ""},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(containers)
	})
	mux.HandleFunc("/v1.43/containers/dead123abc456/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(inspectResp)
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(dockerDiskUsage{})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.FailedContainers) != 1 {
		t.Fatalf("expected 1 failed container, got %d", len(result.FailedContainers))
	}
	if result.FailedContainers[0] != "failed-svc" {
		t.Errorf("expected failed container 'failed-svc', got %q", result.FailedContainers[0])
	}
}

func TestDockerAgent_DiskUsage(t *testing.T) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]dockerContainer{})
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		df := dockerDiskUsage{
			Images: []dockerImageDU{
				{Size: 1073741824, Containers: 1}, // 1 GB, in use
				{Size: 536870912, Containers: 0},   // 0.5 GB, reclaimable
			},
			Containers: []dockerContainerDU{
				{SizeRw: 10485760},
			},
			Volumes: []dockerVolumeDU{
				{UsageData: dockerUsageData{Size: 214748364, RefCount: 0}},
			},
		}
		json.NewEncoder(w).Encode(df)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DiskUsage == "" {
		t.Fatal("expected non-empty DiskUsage")
	}
	if !strings.Contains(result.DiskUsage, "Images: 2") {
		t.Errorf("expected DiskUsage to mention 2 images, got %q", result.DiskUsage)
	}
	if !strings.Contains(result.DiskUsage, "Containers: 1") {
		t.Errorf("expected DiskUsage to mention 1 container, got %q", result.DiskUsage)
	}
	if !strings.Contains(result.DiskUsage, "Volumes: 1") {
		t.Errorf("expected DiskUsage to mention 1 volume, got %q", result.DiskUsage)
	}
}

func TestDockerAgent_ImageIssues(t *testing.T) {
	t.Helper()

	containers := []dockerContainer{
		{ID: "imgfail123456", Names: []string{"/bad-image"}, Image: "nonexistent:latest", State: "dead", Status: "Dead"},
	}

	inspectResp := dockerInspect{
		State: dockerState{ExitCode: 125, OOMKilled: false, Error: "image not found: nonexistent:latest"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(containers)
	})
	mux.HandleFunc("/v1.43/containers/imgfail123456/json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(inspectResp)
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(dockerDiskUsage{})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ImageIssues) != 1 {
		t.Fatalf("expected 1 image issue, got %d", len(result.ImageIssues))
	}
	if !strings.Contains(result.ImageIssues[0], "bad-image") {
		t.Errorf("expected image issue to mention 'bad-image', got %q", result.ImageIssues[0])
	}
	if !strings.Contains(result.ImageIssues[0], "image not found") {
		t.Errorf("expected image issue to mention 'image not found', got %q", result.ImageIssues[0])
	}
}

func TestDockerAgent_ServerError(t *testing.T) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1.43/containers/json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	})
	mux.HandleFunc("/v1.43/system/df", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req := &models.AnalysisRequest{
		Docker: models.DockerConfig{Host: server.URL},
	}
	agent := NewDockerAgent(req)

	result, err := agent.Analyze(context.Background(), &models.BuildContext{})
	if err != nil {
		t.Fatalf("expected graceful handling (no error returned), got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result even on server errors")
	}
	// Container list and disk usage both failed, so result should be mostly empty.
	if len(result.ContainerStatuses) != 0 {
		t.Errorf("expected empty ContainerStatuses on server error, got %d", len(result.ContainerStatuses))
	}
	if result.DiskUsage != "" {
		t.Errorf("expected empty DiskUsage on server error, got %q", result.DiskUsage)
	}
}
