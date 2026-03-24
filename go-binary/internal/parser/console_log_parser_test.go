package parser

import (
	"testing"
)

const sampleConsoleLog = `Started by user admin
Running on jenkins-agent-7b8f9 in /var/lib/jenkins/workspace/MyProject
[Pipeline] {
[Pipeline] stage
[Pipeline] { (Checkout)
Cloning repository https://git.example.com/scm/payments/webapp.git
Checking out Revision abc123def456789abcdef0123456789abcdef0ab (main)
[Pipeline] }
[Pipeline] stage
[Pipeline] { (Build - payments)
Compiling src/main/java/com/example/App.java
ERROR: compilation failed - cannot find symbol
  symbol:   class FooService
  location: class com.example.App
[Pipeline] }
Stage 'Build - payments' failed
[Pipeline] stage
[Pipeline] { (Build - orders)
Building orders module...
[Pipeline] }
ERROR: script returned exit code 1
Finished: FAILURE`

func TestParse_ExtractsRepository(t *testing.T) {
	ctx := Parse(sampleConsoleLog)
	if ctx.Repository != "https://git.example.com/scm/payments/webapp.git" {
		t.Errorf("expected repository URL, got %q", ctx.Repository)
	}
}

func TestParse_ExtractsBranch(t *testing.T) {
	ctx := Parse(sampleConsoleLog)
	if ctx.Branch != "main" {
		t.Errorf("expected branch 'main', got %q", ctx.Branch)
	}
}

func TestParse_ExtractsCommitHash(t *testing.T) {
	ctx := Parse(sampleConsoleLog)
	if ctx.CommitHash != "abc123def456789abcdef0123456789abcdef0ab" {
		t.Errorf("expected commit hash, got %q", ctx.CommitHash)
	}
}

func TestParse_ExtractsAgentName(t *testing.T) {
	ctx := Parse(sampleConsoleLog)
	if ctx.AgentName != "jenkins-agent-7b8f9" {
		t.Errorf("expected agent 'jenkins-agent-7b8f9', got %q", ctx.AgentName)
	}
}

func TestParse_ExtractsFailedStage(t *testing.T) {
	ctx := Parse(sampleConsoleLog)
	if ctx.FailedStage != "Build - payments" {
		t.Errorf("expected failed stage 'Build - payments', got %q", ctx.FailedStage)
	}
}

func TestParse_ExtractsErrorMessages(t *testing.T) {
	ctx := Parse(sampleConsoleLog)
	if len(ctx.ErrorMessages) == 0 {
		t.Fatal("expected error messages, got none")
	}
	found := false
	for _, msg := range ctx.ErrorMessages {
		if msg == "ERROR: compilation failed - cannot find symbol" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected compilation error in messages, got %v", ctx.ErrorMessages)
	}
}

func TestParse_ExtractsSuspectedRepository(t *testing.T) {
	ctx := Parse(sampleConsoleLog)
	if ctx.SuspectedRepository != "payments" {
		t.Errorf("expected suspected repository 'payments', got %q", ctx.SuspectedRepository)
	}
}

func TestParse_ExtractsAllStages(t *testing.T) {
	ctx := Parse(sampleConsoleLog)
	if len(ctx.AllStages) < 2 {
		t.Errorf("expected at least 2 stages, got %d: %v", len(ctx.AllStages), ctx.AllStages)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	ctx := Parse("")
	if ctx == nil {
		t.Fatal("Parse should return non-nil BuildContext for empty input")
	}
	if ctx.FailedStage != "" {
		t.Errorf("expected empty failed stage for empty input, got %q", ctx.FailedStage)
	}
}
