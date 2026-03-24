package correlation

import (
	"testing"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// --- Priority 1: Suspected Repository ---

func TestAnalyze_Priority1_SuspectedRepository_BitBucket(t *testing.T) {
	ctx := &models.BuildContext{
		SuspectedRepository: "payments",
		FailedStage:         "Build - payments",
		ErrorMessages:       []string{"ERROR: compilation failed in src/main/App.java"},
	}
	results := &models.McpResults{
		BitBucketResult: &models.BitBucketAgentResult{
			RecentCommits: []models.CommitInfo{
				{Hash: "abc123", Message: "fix: update App.java", Author: "dev1"},
			},
		},
	}

	corr := Analyze(ctx, results)

	if corr.RootCauseType != "CodeChange" {
		t.Errorf("expected CodeChange, got %q", corr.RootCauseType)
	}
	if corr.IsInfrastructure {
		t.Error("expected IsInfrastructure=false")
	}
	if corr.ResponsibleRepository != "payments" {
		t.Errorf("expected responsible repo 'payments', got %q", corr.ResponsibleRepository)
	}
	if len(corr.Evidence) < 2 {
		t.Errorf("expected at least 2 evidence items (code file + commit), got %d", len(corr.Evidence))
	}
}

func TestAnalyze_Priority1_SuspectedRepository_GitHub(t *testing.T) {
	ctx := &models.BuildContext{
		SuspectedRepository: "my-service",
		ErrorMessages:       []string{"cannot find symbol in Handler.java"},
	}
	results := &models.McpResults{
		GithubResult: &models.GithubAgentResult{
			RecentCommits: []models.CommitInfo{
				{Hash: "def456", Message: "refactor: move handler", Author: "dev2"},
			},
		},
	}

	corr := Analyze(ctx, results)

	if corr.RootCauseType != "CodeChange" {
		t.Errorf("expected CodeChange, got %q", corr.RootCauseType)
	}
	if corr.ResponsibleRepository != "my-service" {
		t.Errorf("expected responsible repo 'my-service', got %q", corr.ResponsibleRepository)
	}
}

func TestAnalyze_Priority1_BothSCMs(t *testing.T) {
	ctx := &models.BuildContext{
		SuspectedRepository: "cross-repo",
		ErrorMessages:       []string{"error in main.go"},
	}
	results := &models.McpResults{
		BitBucketResult: &models.BitBucketAgentResult{
			RecentCommits: []models.CommitInfo{
				{Hash: "bb1", Message: "bb commit", Author: "bb-dev"},
			},
		},
		GithubResult: &models.GithubAgentResult{
			RecentCommits: []models.CommitInfo{
				{Hash: "gh1", Message: "gh commit", Author: "gh-dev"},
			},
		},
	}

	corr := Analyze(ctx, results)

	if corr.RootCauseType != "CodeChange" {
		t.Errorf("expected CodeChange, got %q", corr.RootCauseType)
	}
	// Should have evidence from both SCMs
	if len(corr.Evidence) < 3 {
		t.Errorf("expected at least 3 evidence items (code file + bb commit + gh commit), got %d", len(corr.Evidence))
	}
}

func TestAnalyze_Priority1_NoEvidence(t *testing.T) {
	ctx := &models.BuildContext{
		SuspectedRepository: "some-repo",
		ErrorMessages:       []string{"generic error with no code file reference"},
	}
	results := &models.McpResults{}

	corr := Analyze(ctx, results)

	// Should fall through to default since no code file extensions or commits
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure fallthrough, got %q", corr.RootCauseType)
	}
}

// --- Priority 2: Build Stage Failure ---

func TestAnalyze_Priority2_BuildStage(t *testing.T) {
	ctx := &models.BuildContext{
		SuspectedRepository: "",
		FailedStage:         "Build - orders",
	}
	results := &models.McpResults{}

	corr := Analyze(ctx, results)

	if corr.RootCauseType != "CodeChange" {
		t.Errorf("expected CodeChange, got %q", corr.RootCauseType)
	}
	if corr.IsInfrastructure {
		t.Error("expected IsInfrastructure=false")
	}
	if corr.ResponsibleRepository != "orders" {
		t.Errorf("expected responsible repo 'orders', got %q", corr.ResponsibleRepository)
	}
}

func TestAnalyze_Priority2_BuildStage_PrefixPattern(t *testing.T) {
	ctx := &models.BuildContext{
		FailedStage: "E3 Build",
	}
	results := &models.McpResults{}

	corr := Analyze(ctx, results)
	if corr.RootCauseType != "CodeChange" {
		t.Errorf("expected CodeChange, got %q", corr.RootCauseType)
	}
	if corr.ResponsibleRepository != "payments" {
		t.Errorf("expected 'payments', got %q", corr.ResponsibleRepository)
	}
}

func TestAnalyze_Priority2_NonBuildStage(t *testing.T) {
	ctx := &models.BuildContext{
		FailedStage: "Deploy to Production",
	}
	results := &models.McpResults{}

	corr := Analyze(ctx, results)
	// Should not match build stage, falls to default
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
}

// --- Priority 3: Kubernetes Issues ---

func TestAnalyze_Priority3_OOMKills(t *testing.T) {
	ctx := &models.BuildContext{
		FailedStage: "Deploy to Staging",
	}
	results := &models.McpResults{
		KubernetesResult: &models.KubernetesAgentResult{
			OOMKills: []string{"pod-abc/container1"},
		},
	}

	corr := Analyze(ctx, results)

	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
	if !corr.IsInfrastructure {
		t.Error("expected IsInfrastructure=true")
	}
	if corr.ResponsibleTeam != "DevOps" {
		t.Errorf("expected DevOps, got %q", corr.ResponsibleTeam)
	}
}

func TestAnalyze_Priority3_NodePressure(t *testing.T) {
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		KubernetesResult: &models.KubernetesAgentResult{
			NodePressure: true,
		},
	}

	corr := Analyze(ctx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
}

func TestAnalyze_Priority3_K8sHealthy(t *testing.T) {
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		KubernetesResult: &models.KubernetesAgentResult{
			OOMKills:     []string{},
			NodePressure: false,
		},
	}

	corr := Analyze(ctx, results)
	// Healthy K8s should fall through to default
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure default, got %q", corr.RootCauseType)
	}
}

// --- Priority 4: Docker Issues ---

func TestAnalyze_Priority4_DockerOOMKill(t *testing.T) {
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			OOMKilled: []string{"my-app-container"},
		},
	}

	corr := Analyze(ctx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
	if !corr.IsInfrastructure {
		t.Error("expected IsInfrastructure=true")
	}
	if corr.ResponsibleTeam != "DevOps" {
		t.Errorf("expected DevOps, got %q", corr.ResponsibleTeam)
	}
}

func TestAnalyze_Priority4_DockerFailed(t *testing.T) {
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			FailedContainers: []string{"build-runner exited with code 1"},
		},
	}

	corr := Analyze(ctx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
}

func TestAnalyze_Priority4_DockerMixed(t *testing.T) {
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			OOMKilled:        []string{"oom-container"},
			FailedContainers: []string{"exit-container"},
		},
	}

	corr := Analyze(ctx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
	if len(corr.Evidence) < 2 {
		t.Errorf("expected at least 2 evidence items, got %d", len(corr.Evidence))
	}
}

func TestAnalyze_Priority4_DockerHealthy(t *testing.T) {
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			FailedContainers: []string{},
			OOMKilled:        []string{},
		},
	}

	corr := Analyze(ctx, results)
	// Healthy Docker should fall through
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure default, got %q", corr.RootCauseType)
	}
}

// --- Priority 5: JFrog Issues ---

func TestAnalyze_Priority5_JFrogMissing(t *testing.T) {
	ctx := &models.BuildContext{
		FailedStage: "Resolve Dependencies",
	}
	results := &models.McpResults{
		JFrogResult: &models.JFrogAgentResult{
			ArtifactsAvailable: false,
			MissingArtifacts:   []string{"com.company:lib:1.0"},
		},
	}

	corr := Analyze(ctx, results)
	if corr.RootCauseType != "DependencyIssue" {
		t.Errorf("expected DependencyIssue, got %q", corr.RootCauseType)
	}
}

func TestAnalyze_Priority5_JFrogAvailable(t *testing.T) {
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		JFrogResult: &models.JFrogAgentResult{
			ArtifactsAvailable: true,
			MissingArtifacts:   []string{},
		},
	}

	corr := Analyze(ctx, results)
	// Available artifacts should fall through
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure default, got %q", corr.RootCauseType)
	}
}

// --- Priority 6: Nexus Issues ---

func TestAnalyze_Priority6_NexusMissing(t *testing.T) {
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		NexusResult: &models.NexusAgentResult{
			ArtifactsAvailable: false,
			MissingArtifacts:   []string{"org.example:core:2.0.0", "org.example:utils:2.0.0"},
		},
	}

	corr := Analyze(ctx, results)
	if corr.RootCauseType != "DependencyIssue" {
		t.Errorf("expected DependencyIssue, got %q", corr.RootCauseType)
	}
	if len(corr.Evidence) != 2 {
		t.Errorf("expected 2 evidence items, got %d", len(corr.Evidence))
	}
}

func TestAnalyze_Priority6_NexusAvailable(t *testing.T) {
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		NexusResult: &models.NexusAgentResult{
			ArtifactsAvailable: true,
		},
	}

	corr := Analyze(ctx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure default, got %q", corr.RootCauseType)
	}
}

// --- Priority 7: Default ---

func TestAnalyze_Priority7_Default_Infrastructure(t *testing.T) {
	ctx := &models.BuildContext{
		FailedStage: "Cleanup",
	}
	results := &models.McpResults{}

	corr := Analyze(ctx, results)

	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure, got %q", corr.RootCauseType)
	}
	if !corr.IsInfrastructure {
		t.Error("expected IsInfrastructure=true")
	}
	if corr.ResponsibleTeam != "DevOps" {
		t.Errorf("expected DevOps team, got %q", corr.ResponsibleTeam)
	}
}

func TestAnalyze_NilResults(t *testing.T) {
	ctx := &models.BuildContext{}
	corr := Analyze(ctx, nil)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure with nil results, got %q", corr.RootCauseType)
	}
}

func TestAnalyze_NilBuildContext(t *testing.T) {
	ctx := &models.BuildContext{}
	results := &models.McpResults{}
	// Empty context should not panic
	corr := Analyze(ctx, results)
	if corr == nil {
		t.Fatal("expected non-nil correlation")
	}
}

// --- Priority Ordering Tests ---

func TestAnalyze_K8sBeatsDocker(t *testing.T) {
	// K8s (priority 3) should take precedence over Docker (priority 4)
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		KubernetesResult: &models.KubernetesAgentResult{
			OOMKills: []string{"k8s-pod-oom"},
		},
		DockerResult: &models.DockerAgentResult{
			OOMKilled: []string{"docker-oom"},
		},
	}

	corr := Analyze(ctx, results)
	// Should match K8s first
	found := false
	for _, e := range corr.Evidence {
		if e == "OOM kill detected: k8s-pod-oom" {
			found = true
		}
	}
	if !found {
		t.Error("expected K8s OOM evidence (higher priority), got Docker evidence instead")
	}
}

func TestAnalyze_DockerBeatsJFrog(t *testing.T) {
	// Docker (priority 4) should take precedence over JFrog (priority 5)
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			FailedContainers: []string{"docker-fail"},
		},
		JFrogResult: &models.JFrogAgentResult{
			ArtifactsAvailable: false,
			MissingArtifacts:   []string{"some:artifact:1.0"},
		},
	}

	corr := Analyze(ctx, results)
	if corr.RootCauseType != "Infrastructure" {
		t.Errorf("expected Infrastructure (Docker), got %q", corr.RootCauseType)
	}
}

func TestAnalyze_JFrogBeatsNexus(t *testing.T) {
	// JFrog (priority 5) should take precedence over Nexus (priority 6)
	ctx := &models.BuildContext{}
	results := &models.McpResults{
		JFrogResult: &models.JFrogAgentResult{
			ArtifactsAvailable: false,
			MissingArtifacts:   []string{"jfrog:artifact"},
		},
		NexusResult: &models.NexusAgentResult{
			ArtifactsAvailable: false,
			MissingArtifacts:   []string{"nexus:artifact"},
		},
	}

	corr := Analyze(ctx, results)
	// Should contain JFrog evidence
	found := false
	for _, e := range corr.Evidence {
		if e == "Missing artifact: jfrog:artifact" {
			found = true
		}
	}
	if !found {
		t.Error("expected JFrog evidence (higher priority)")
	}
}

// --- Helper Function Tests ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestExtractRepoFromStageName(t *testing.T) {
	tests := []struct {
		stage string
		want  string
	}{
		{"Build - orders", "orders"},
		{"Build - payments", "payments"},
		{"payments Build", "payments"},
		{"Deploy - payments", ""},
		{"Build", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractRepoFromStageName(tt.stage)
		if got != tt.want {
			t.Errorf("extractRepoFromStageName(%q) = %q, want %q", tt.stage, got, tt.want)
		}
	}
}
