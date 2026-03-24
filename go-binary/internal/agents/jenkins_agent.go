package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// JenkinsAgent queries the Jenkins REST API to collect pipeline stage
// information and build-agent health metrics.
type JenkinsAgent struct {
	req *models.AnalysisRequest
}

// NewJenkinsAgent creates a JenkinsAgent bound to the given analysis request.
func NewJenkinsAgent(req *models.AnalysisRequest) *JenkinsAgent {
	return &JenkinsAgent{req: req}
}

// Analyze fetches pipeline stage data and agent metrics from Jenkins.
func (a *JenkinsAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.JenkinsAgentResult, error) {
	result := &models.JenkinsAgentResult{
		AgentOnline: true, // assume online unless proven otherwise
	}

	// --- Fetch pipeline stages via Workflow API ---
	stages, err := a.fetchStages(ctx)
	if err != nil {
		log.Printf("[JenkinsAgent] failed to fetch stages: %v", err)
	} else {
		result.Stages = stages
	}

	// --- Fetch agent info ---
	if buildCtx.AgentName != "" {
		online, diskFree, err := a.fetchAgentInfo(ctx, buildCtx.AgentName)
		if err != nil {
			log.Printf("[JenkinsAgent] failed to fetch agent info for %s: %v", buildCtx.AgentName, err)
			// Keep partial results; don't fail the whole analysis.
		} else {
			result.AgentOnline = online
			result.AgentDiskFree = diskFree
		}
	}

	return result, nil
}

// fetchStages calls the Jenkins Workflow API to retrieve pipeline stage details.
func (a *JenkinsAgent) fetchStages(ctx context.Context) ([]models.StageInfo, error) {
	url := fmt.Sprintf("%s/job/%s/%d/wfapi/describe",
		strings.TrimRight(a.req.JenkinsUrl, "/"),
		a.req.JobName,
		a.req.BuildNumber,
	)

	body, err := a.doJenkinsRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	var desc wfapiDescribe
	if err := json.Unmarshal(body, &desc); err != nil {
		return nil, fmt.Errorf("parsing wfapi response: %w", err)
	}

	stages := make([]models.StageInfo, 0, len(desc.Stages))
	for _, s := range desc.Stages {
		stages = append(stages, models.StageInfo{
			Name:       s.Name,
			Status:     s.Status,
			DurationMs: s.DurationMillis,
		})
	}
	return stages, nil
}

// fetchAgentInfo queries the Jenkins computer API for agent online status and
// disk space information.
func (a *JenkinsAgent) fetchAgentInfo(ctx context.Context, agentName string) (online bool, diskFree string, err error) {
	url := fmt.Sprintf("%s/computer/%s/api/json?tree=offline,monitorData[*]",
		strings.TrimRight(a.req.JenkinsUrl, "/"),
		agentName,
	)

	body, err := a.doJenkinsRequest(ctx, url)
	if err != nil {
		return false, "", err
	}

	var info agentInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return false, "", fmt.Errorf("parsing agent info: %w", err)
	}

	online = !info.Offline
	diskFree = extractDiskFree(info.MonitorData)
	return online, diskFree, nil
}

// doJenkinsRequest performs an authenticated GET against the Jenkins instance.
func (a *JenkinsAgent) doJenkinsRequest(ctx context.Context, url string) ([]byte, error) {
	// Jenkins uses the job-specific API token or user password for basic auth.
	// The username/password are embedded in the JenkinsUrl or provided via a
	// crumb header; here we use URL-based basic auth as a common pattern.
	// For simplicity we pass no separate auth header — the URL may contain
	// credentials, or Jenkins may be behind a reverse proxy that handles auth.
	return doRequest(ctx, url, "", "")
}

// --- internal JSON shapes ------------------------------------------------

type wfapiDescribe struct {
	Stages []wfapiStage `json:"stages"`
}

type wfapiStage struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	DurationMillis int64  `json:"durationMillis"`
}

type agentInfo struct {
	Offline     bool                   `json:"offline"`
	MonitorData map[string]interface{} `json:"monitorData"`
}

// extractDiskFree pulls available disk space from the Jenkins monitor data map.
func extractDiskFree(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	// Jenkins reports disk space under "hudson.node_monitors.DiskSpaceMonitor".
	key := "hudson.node_monitors.DiskSpaceMonitor"
	raw, ok := data[key]
	if !ok {
		return ""
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return ""
	}
	if size, ok := m["size"].(float64); ok {
		gb := size / (1024 * 1024 * 1024)
		return fmt.Sprintf("%.2f GB", gb)
	}
	return fmt.Sprintf("%v", m)
}
