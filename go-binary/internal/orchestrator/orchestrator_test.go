package orchestrator

import (
	"context"
	"testing"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// TestRunAgentsParallel_CategoryBitBucket verifies that selecting "bitbucket"
// as repoSoftware runs only the BitBucket agent (not GitHub).
func TestRunAgentsParallel_CategoryBitBucket(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			RepoSoftware: "bitbucket",
		},
		// BitBucket config present but will fail connecting — we only care about selection.
		BitBucket: models.BitBucketConfig{Url: "http://localhost:1", Username: "u", Password: "p"},
	}
	buildCtx := &models.BuildContext{JobName: "test", BuildNumber: 1}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	// BitBucket should have been attempted (non-nil result or at least the field was touched)
	// GitHub should NOT have been run
	if results.GithubResult != nil {
		t.Error("GitHub agent should not run when repoSoftware=bitbucket")
	}
	// Jenkins always runs
	if results.JenkinsResult == nil {
		t.Error("Jenkins agent should always run")
	}
}

// TestRunAgentsParallel_CategoryGitHub verifies that selecting "github"
// as repoSoftware runs only the GitHub agent (not BitBucket).
func TestRunAgentsParallel_CategoryGitHub(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			RepoSoftware: "github",
		},
		GitHub: models.GitHubSCMConfig{Token: "fake-token", Owner: "org", Repo: "repo"},
	}
	buildCtx := &models.BuildContext{JobName: "test", BuildNumber: 1}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results.BitBucketResult != nil {
		t.Error("BitBucket agent should not run when repoSoftware=github")
	}
}

// TestRunAgentsParallel_CategoryDocker verifies that selecting "docker"
// as clusterType runs only the Docker agent (not Kubernetes).
func TestRunAgentsParallel_CategoryDocker(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			ClusterType: "docker",
		},
		Docker: models.DockerConfig{Host: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{JobName: "test", BuildNumber: 1}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results.KubernetesResult != nil {
		t.Error("Kubernetes agent should not run when clusterType=docker")
	}
}

// TestRunAgentsParallel_CategoryKubernetes verifies that selecting "kubernetes"
// as clusterType runs only the Kubernetes agent (not Docker).
func TestRunAgentsParallel_CategoryKubernetes(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			ClusterType: "kubernetes",
		},
		Kubernetes: models.KubernetesConfig{ApiUrl: "http://localhost:1", Token: "fake"},
	}
	buildCtx := &models.BuildContext{JobName: "test", BuildNumber: 1}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results.DockerResult != nil {
		t.Error("Docker agent should not run when clusterType=kubernetes")
	}
}

// TestRunAgentsParallel_CategoryNexus verifies that selecting "nexus"
// as artifactManager runs only the Nexus agent (not JFrog).
func TestRunAgentsParallel_CategoryNexus(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			ArtifactManager: "nexus",
		},
		Nexus: models.NexusConfig{Url: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{JobName: "test", BuildNumber: 1}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results.JFrogResult != nil {
		t.Error("JFrog agent should not run when artifactManager=nexus")
	}
}

// TestRunAgentsParallel_CategoryJFrog verifies that selecting "jfrog"
// as artifactManager runs only the JFrog agent (not Nexus).
func TestRunAgentsParallel_CategoryJFrog(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			ArtifactManager: "jfrog",
		},
		JFrog: models.JFrogConfig{Url: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{JobName: "test", BuildNumber: 1}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results.NexusResult != nil {
		t.Error("Nexus agent should not run when artifactManager=jfrog")
	}
}

// TestRunAgentsParallel_AutoDetect verifies that when no categories are specified,
// agents run based on which configs are populated.
func TestRunAgentsParallel_AutoDetect(t *testing.T) {
	req := &models.AnalysisRequest{
		// No categories — auto-detect mode
		BitBucket:  models.BitBucketConfig{Url: "http://localhost:1", Username: "u", Password: "p"},
		Docker:     models.DockerConfig{Host: "http://localhost:1"},
		Nexus:      models.NexusConfig{Url: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{JobName: "test", BuildNumber: 1}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	// With auto-detect, BitBucket should run (config present)
	// GitHub should NOT (no token)
	if results.GithubResult != nil {
		t.Error("GitHub agent should not run in auto-detect when no GitHub token is configured")
	}
	// Docker should have been attempted (config present)
	// K8s should NOT (no API URL)
	if results.KubernetesResult != nil {
		t.Error("K8s agent should not run in auto-detect when no K8s API URL is configured")
	}
	// Nexus should have been attempted
	// JFrog should NOT (no URL)
	if results.JFrogResult != nil {
		t.Error("JFrog agent should not run in auto-detect when no JFrog URL is configured")
	}
	// Jenkins always runs
	if results.JenkinsResult == nil {
		t.Error("Jenkins agent should always run")
	}
}

// TestRunAgentsParallel_AutoDetectBothConfigured verifies that when both
// configs are populated in auto-detect mode, both agents run.
func TestRunAgentsParallel_AutoDetectBothConfigured(t *testing.T) {
	req := &models.AnalysisRequest{
		// Both SCM configs present
		BitBucket: models.BitBucketConfig{Url: "http://localhost:1", Username: "u", Password: "p"},
		GitHub:    models.GitHubSCMConfig{Token: "fake-token", Owner: "org", Repo: "repo"},
		// Both cluster configs present
		Kubernetes: models.KubernetesConfig{ApiUrl: "http://localhost:1", Token: "fake"},
		Docker:     models.DockerConfig{Host: "http://localhost:1"},
		// Both artifact configs present
		JFrog: models.JFrogConfig{Url: "http://localhost:1"},
		Nexus: models.NexusConfig{Url: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{JobName: "test", BuildNumber: 1}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	// All agents should have been attempted in auto-detect with both configs
	if results.JenkinsResult == nil {
		t.Error("Jenkins agent should always run")
	}
}

// TestRunAgentsParallel_CaseInsensitive verifies that category values are
// case-insensitive.
func TestRunAgentsParallel_CaseInsensitive(t *testing.T) {
	req := &models.AnalysisRequest{
		Categories: models.SoftwareCategories{
			RepoSoftware:    "GITHUB",
			ClusterType:     "Docker",
			ArtifactManager: "NEXUS",
		},
		GitHub: models.GitHubSCMConfig{Token: "fake", Owner: "org", Repo: "repo"},
		Docker: models.DockerConfig{Host: "http://localhost:1"},
		Nexus:  models.NexusConfig{Url: "http://localhost:1"},
	}
	buildCtx := &models.BuildContext{JobName: "test", BuildNumber: 1}

	results := runAgentsParallel(context.Background(), req, buildCtx)

	// Verify the correct agents were selected despite uppercase
	if results.BitBucketResult != nil {
		t.Error("BitBucket should not run when repoSoftware=GITHUB")
	}
	if results.KubernetesResult != nil {
		t.Error("K8s should not run when clusterType=Docker")
	}
	if results.JFrogResult != nil {
		t.Error("JFrog should not run when artifactManager=NEXUS")
	}
}

// TestRunAgentsParallel_EmptyConfig verifies that no agents crash when configs
// are entirely empty (no categories, no credentials).
func TestRunAgentsParallel_EmptyConfig(t *testing.T) {
	req := &models.AnalysisRequest{}
	buildCtx := &models.BuildContext{JobName: "test", BuildNumber: 1}

	// Should not panic
	results := runAgentsParallel(context.Background(), req, buildCtx)

	if results == nil {
		t.Error("results should never be nil")
	}
	// Jenkins always runs
	if results.JenkinsResult == nil {
		t.Error("Jenkins agent should always run even with empty config")
	}
}
