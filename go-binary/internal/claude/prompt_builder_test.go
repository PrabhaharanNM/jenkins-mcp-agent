package claude

import (
	"strings"
	"testing"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

func TestBuildUserPrompt_BasicOutput(t *testing.T) {
	ctx := &models.BuildContext{
		JobName:       "MyProject",
		BuildNumber:   42,
		BuildUrl:      "http://jenkins/job/MyProject/42",
		FailedStage:   "Build - E3",
		AgentName:     "agent-1",
		ErrorMessages: []string{"ERROR: compilation failed"},
		ConsoleLog:    "some log output\nERROR: compilation failed\nFinished: FAILURE",
	}
	results := &models.McpResults{}
	corr := &models.Correlation{
		RootCauseType:    "CodeChange",
		IsInfrastructure: false,
	}

	prompt := BuildUserPrompt(ctx, results, corr)

	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !strings.Contains(prompt, "MyProject") {
		t.Error("prompt should contain job name")
	}
	if !strings.Contains(prompt, "Build - E3") {
		t.Error("prompt should contain failed stage")
	}
	if !strings.Contains(prompt, "compilation failed") {
		t.Error("prompt should contain error messages")
	}
}

func TestBuildUserPrompt_NilResults(t *testing.T) {
	ctx := &models.BuildContext{
		JobName:     "TestJob",
		BuildNumber: 1,
	}

	// Should not panic with nil results
	prompt := BuildUserPrompt(ctx, nil, nil)

	if prompt == "" {
		t.Fatal("expected non-empty prompt even with nil results")
	}
	if !strings.Contains(prompt, "TestJob") {
		t.Error("prompt should contain job name")
	}
	// Check all "Data not available" sections are present
	sections := []string{
		"JENKINS AGENT DATA",
		"BITBUCKET DATA",
		"GITHUB DATA",
		"KUBERNETES DATA",
		"DOCKER DATA",
		"JFROG DATA",
		"NEXUS DATA",
		"CROSS-CORRELATION ANALYSIS",
	}
	for _, section := range sections {
		if !strings.Contains(prompt, section) {
			t.Errorf("prompt should contain section %q", section)
		}
	}
}

func TestBuildUserPrompt_WithAllAgentResults(t *testing.T) {
	ctx := &models.BuildContext{
		JobName:     "FullTest",
		BuildNumber: 99,
		FailedStage: "Build - AP",
	}
	results := &models.McpResults{
		JenkinsResult: &models.JenkinsAgentResult{
			Stages: []models.StageInfo{{Name: "Build - AP", Status: "FAILED"}},
		},
		BitBucketResult: &models.BitBucketAgentResult{
			CodeOwners: "* @team-ap",
			RecentCommits: []models.CommitInfo{
				{Hash: "abc123", Message: "fix: something", Author: "dev1", Date: "2024-01-01"},
			},
		},
		KubernetesResult: &models.KubernetesAgentResult{
			OOMKills: []string{},
		},
		JFrogResult: &models.JFrogAgentResult{
			ArtifactsAvailable: true,
		},
	}
	corr := &models.Correlation{
		RootCauseType: "CodeChange",
	}

	prompt := BuildUserPrompt(ctx, results, corr)

	if !strings.Contains(prompt, "FAILED") {
		t.Error("prompt should contain stage status")
	}
	if !strings.Contains(prompt, "team-ap") {
		t.Error("prompt should contain CODEOWNERS data")
	}
}

// --- Section-specific tests ---

func TestBuildUserPrompt_GitHubDataSection(t *testing.T) {
	ctx := &models.BuildContext{JobName: "Test", BuildNumber: 1}
	results := &models.McpResults{
		GithubResult: &models.GithubAgentResult{
			PrTitle:    "Fix: critical bug in auth",
			PrBody:     "This fixes the login issue",
			CodeOwners: "* @backend-team",
			RecentCommits: []models.CommitInfo{
				{Hash: "abc12345def", Message: "fix auth flow", Author: "dev1", Date: "2024-03-01"},
			},
			ChangedFiles: []string{"src/auth.go", "src/auth_test.go"},
		},
	}

	prompt := BuildUserPrompt(ctx, results, nil)

	if !strings.Contains(prompt, "GITHUB DATA") {
		t.Error("prompt should contain GITHUB DATA section")
	}
	if !strings.Contains(prompt, "Fix: critical bug in auth") {
		t.Error("prompt should contain PR title")
	}
	if !strings.Contains(prompt, "fix auth flow") {
		t.Error("prompt should contain commit message")
	}
	if !strings.Contains(prompt, "src/auth.go") {
		t.Error("prompt should contain changed files")
	}
	if !strings.Contains(prompt, "backend-team") {
		t.Error("prompt should contain CODEOWNERS")
	}
	// Verify hash is shortened to 8 chars
	if !strings.Contains(prompt, "abc12345") {
		t.Error("prompt should contain shortened hash")
	}
}

func TestBuildUserPrompt_DockerDataSection(t *testing.T) {
	ctx := &models.BuildContext{JobName: "Test", BuildNumber: 1}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			ContainerStatuses: []models.ContainerStatus{
				{Name: "web-app", Image: "nginx:latest", State: "running", Status: "Up 2 hours", ExitCode: 0},
				{Name: "worker", Image: "myapp:v1", State: "exited", Status: "Exited (137)", ExitCode: 137},
			},
			FailedContainers: []string{"worker exited(137)"},
			OOMKilled:        []string{"worker"},
			ImageIssues:      []string{"myapp:v2 not found"},
			DiskUsage:        "45.2GB / 100GB",
		},
	}

	prompt := BuildUserPrompt(ctx, results, nil)

	if !strings.Contains(prompt, "DOCKER DATA") {
		t.Error("prompt should contain DOCKER DATA section")
	}
	if !strings.Contains(prompt, "web-app") {
		t.Error("prompt should contain container name")
	}
	if !strings.Contains(prompt, "nginx:latest") {
		t.Error("prompt should contain image name")
	}
	if !strings.Contains(prompt, "Failed Containers") {
		t.Error("prompt should contain failed containers section")
	}
	if !strings.Contains(prompt, "OOM Killed") {
		t.Error("prompt should contain OOM killed section")
	}
	if !strings.Contains(prompt, "Image Issues") {
		t.Error("prompt should contain image issues")
	}
	if !strings.Contains(prompt, "45.2GB") {
		t.Error("prompt should contain disk usage")
	}
}

func TestBuildUserPrompt_NexusDataSection(t *testing.T) {
	ctx := &models.BuildContext{JobName: "Test", BuildNumber: 1}
	results := &models.McpResults{
		NexusResult: &models.NexusAgentResult{
			ArtifactsAvailable: false,
			RepositoryStatus:   "healthy",
			MissingArtifacts:   []string{"org.example:core:2.0.0", "org.example:utils:2.0.0"},
		},
	}

	prompt := BuildUserPrompt(ctx, results, nil)

	if !strings.Contains(prompt, "NEXUS DATA") {
		t.Error("prompt should contain NEXUS DATA section")
	}
	if !strings.Contains(prompt, "Artifacts Available: false") {
		t.Error("prompt should show artifacts not available")
	}
	if !strings.Contains(prompt, "org.example:core:2.0.0") {
		t.Error("prompt should contain missing artifact")
	}
	if !strings.Contains(prompt, "healthy") {
		t.Error("prompt should contain repository status")
	}
}

func TestBuildUserPrompt_DockerNilSubfields(t *testing.T) {
	ctx := &models.BuildContext{JobName: "Test", BuildNumber: 1}
	results := &models.McpResults{
		DockerResult: &models.DockerAgentResult{
			// All slices nil/empty
		},
	}

	prompt := BuildUserPrompt(ctx, results, nil)
	if !strings.Contains(prompt, "DOCKER DATA") {
		t.Error("prompt should contain DOCKER DATA section")
	}
	// Should not contain subsection headers when data is empty
	if strings.Contains(prompt, "Failed Containers") {
		t.Error("prompt should not contain Failed Containers when empty")
	}
}

func TestBuildUserPrompt_ConsoleLogTruncation(t *testing.T) {
	// Build a console log with > 200 lines
	var lines []string
	for i := 0; i < 300; i++ {
		lines = append(lines, "log line "+string(rune('0'+i%10)))
	}
	ctx := &models.BuildContext{
		JobName:     "TruncTest",
		BuildNumber: 1,
		ConsoleLog:  strings.Join(lines, "\n"),
	}

	prompt := BuildUserPrompt(ctx, nil, nil)
	if !strings.Contains(prompt, "truncated") {
		t.Error("prompt should indicate truncation for >200 lines")
	}
}

func TestBuildUserPrompt_ErrorMessageLimit(t *testing.T) {
	var errors []string
	for i := 0; i < 30; i++ {
		errors = append(errors, "error message number "+string(rune('0'+i%10)))
	}
	ctx := &models.BuildContext{
		JobName:       "ErrorLimit",
		BuildNumber:   1,
		ErrorMessages: errors,
	}

	prompt := BuildUserPrompt(ctx, nil, nil)
	if !strings.Contains(prompt, "and 10 more") {
		t.Error("prompt should indicate truncated error messages")
	}
}

func TestBuildUserPrompt_CorrelationSection(t *testing.T) {
	ctx := &models.BuildContext{JobName: "Test", BuildNumber: 1}
	corr := &models.Correlation{
		RootCauseType:         "DependencyIssue",
		IsInfrastructure:      false,
		ResponsibleRepository: "myapp",
		ResponsibleTeam:       "Platform",
		Evidence:              []string{"Missing artifact: com.example:lib:1.0", "Nexus unhealthy"},
	}

	prompt := BuildUserPrompt(ctx, nil, corr)

	if !strings.Contains(prompt, "DependencyIssue") {
		t.Error("prompt should contain root cause type")
	}
	if !strings.Contains(prompt, "Platform") {
		t.Error("prompt should contain responsible team")
	}
	if !strings.Contains(prompt, "Missing artifact") {
		t.Error("prompt should contain evidence")
	}
}

func TestBuildUserPrompt_AllSectionsPresent(t *testing.T) {
	ctx := &models.BuildContext{
		JobName:     "AllSections",
		BuildNumber: 1,
		FailedStage: "Test",
		ConsoleLog:  "some log",
	}
	results := &models.McpResults{
		JenkinsResult:    &models.JenkinsAgentResult{},
		BitBucketResult:  &models.BitBucketAgentResult{},
		GithubResult:     &models.GithubAgentResult{},
		KubernetesResult: &models.KubernetesAgentResult{},
		DockerResult:     &models.DockerAgentResult{},
		JFrogResult:      &models.JFrogAgentResult{},
		NexusResult:      &models.NexusAgentResult{},
	}
	corr := &models.Correlation{RootCauseType: "Unknown"}

	prompt := BuildUserPrompt(ctx, results, corr)

	requiredSections := []string{
		"BUILD INFORMATION",
		"FAILED STAGE",
		"ERROR MESSAGES",
		"CONSOLE LOG",
		"JENKINS AGENT DATA",
		"BITBUCKET DATA",
		"GITHUB DATA",
		"KUBERNETES DATA",
		"DOCKER DATA",
		"JFROG DATA",
		"NEXUS DATA",
		"CROSS-CORRELATION ANALYSIS",
	}
	for _, section := range requiredSections {
		if !strings.Contains(prompt, section) {
			t.Errorf("prompt missing required section: %s", section)
		}
	}
}

func TestShortHash(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc123def456789", "abc123de"},
		{"short", "short"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
		{"", ""},
	}
	for _, tt := range tests {
		got := shortHash(tt.input)
		if got != tt.want {
			t.Errorf("shortHash(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
