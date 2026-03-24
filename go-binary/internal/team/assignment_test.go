package team

import (
	"testing"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

const testTeamMappings = `{
	"payments": {"name": "Payments Team Lead", "email": "payments@example.com", "jiraUsername": "payments-lead"},
	"orders": {"name": "Orders Team Lead", "email": "orders@example.com", "jiraUsername": "orders-lead"}
}`

const testDevopsManager = `{"name": "Platform Team Lead", "email": "platform@example.com", "jiraUsername": "platform-lead"}`

func TestAssign_InfrastructureIssue_ReturnsDevOps(t *testing.T) {
	req := &models.AnalysisRequest{
		TeamMappings:  testTeamMappings,
		DevopsManager: testDevopsManager,
	}
	ctx := &models.BuildContext{FailedStage: "Deploy"}
	corr := &models.Correlation{
		IsInfrastructure: true,
		RootCauseType:    "Infrastructure",
	}

	tm := Assign(req, ctx, corr)

	if tm.Name != "Platform Team Lead" {
		t.Errorf("expected 'Platform Team Lead', got %q", tm.Name)
	}
	if tm.Email != "platform@example.com" {
		t.Errorf("expected 'platform@example.com', got %q", tm.Email)
	}
}

func TestAssign_KnownRepository_ReturnsTeamManager(t *testing.T) {
	req := &models.AnalysisRequest{
		TeamMappings:  testTeamMappings,
		DevopsManager: testDevopsManager,
	}
	ctx := &models.BuildContext{FailedStage: "Build - payments"}
	corr := &models.Correlation{
		IsInfrastructure:      false,
		ResponsibleRepository: "payments",
	}

	tm := Assign(req, ctx, corr)

	if tm.Name != "Payments Team Lead" {
		t.Errorf("expected 'Payments Team Lead', got %q", tm.Name)
	}
	if tm.JiraUsername != "payments-lead" {
		t.Errorf("expected 'payments-lead', got %q", tm.JiraUsername)
	}
}

func TestAssign_StageNameExtraction_Orders(t *testing.T) {
	req := &models.AnalysisRequest{
		TeamMappings:  testTeamMappings,
		DevopsManager: testDevopsManager,
	}
	ctx := &models.BuildContext{FailedStage: "Build - orders"}
	corr := &models.Correlation{
		IsInfrastructure:      false,
		ResponsibleRepository: "",
	}

	tm := Assign(req, ctx, corr)

	if tm.Name != "Orders Team Lead" {
		t.Errorf("expected 'Orders Team Lead', got %q", tm.Name)
	}
}

func TestAssign_UnknownRepo_FallsBackToDevOps(t *testing.T) {
	req := &models.AnalysisRequest{
		TeamMappings:  testTeamMappings,
		DevopsManager: testDevopsManager,
	}
	ctx := &models.BuildContext{FailedStage: "Unknown Stage"}
	corr := &models.Correlation{
		IsInfrastructure:      false,
		ResponsibleRepository: "",
	}

	tm := Assign(req, ctx, corr)

	if tm.Name != "Platform Team Lead" {
		t.Errorf("expected fallback to 'Platform Team Lead', got %q", tm.Name)
	}
}

func TestAssign_EmptyConfig_ReturnsDefault(t *testing.T) {
	req := &models.AnalysisRequest{
		TeamMappings:  "{}",
		DevopsManager: "{}",
	}
	ctx := &models.BuildContext{FailedStage: "Build - payments"}
	corr := &models.Correlation{
		IsInfrastructure:      false,
		ResponsibleRepository: "payments",
	}

	tm := Assign(req, ctx, corr)

	// Should not panic, return some default
	if tm == nil {
		t.Fatal("expected non-nil TeamManager")
	}
}
