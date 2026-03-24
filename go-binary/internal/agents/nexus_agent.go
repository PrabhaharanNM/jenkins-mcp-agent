package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// NexusAgent queries the Sonatype Nexus REST API to check whether build
// artifacts exist and to assess repository health.
type NexusAgent struct {
	req *models.AnalysisRequest
}

// NewNexusAgent creates a NexusAgent bound to the given request.
func NewNexusAgent(req *models.AnalysisRequest) *NexusAgent {
	return &NexusAgent{req: req}
}

// Analyze searches Nexus for build artifacts derived from the job name. If
// Nexus configuration is not provided, it returns an empty result.
func (a *NexusAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.NexusAgentResult, error) {
	cfg := a.req.Nexus
	if cfg.Url == "" {
		log.Printf("[NexusAgent] Nexus config not provided, skipping")
		return &models.NexusAgentResult{}, nil
	}

	result := &models.NexusAgentResult{
		ArtifactsAvailable: true,
		RepositoryStatus:   "OK",
	}

	baseURL := strings.TrimRight(cfg.Url, "/")
	// Use auth only if password is non-empty; otherwise try anonymous access.
	var auth string
	if cfg.Password != "" {
		auth = basicAuthValue(cfg.Username, cfg.Password)
	}

	// Derive artifact name from the job name (last segment after '/').
	artifactName := a.deriveArtifactName(buildCtx.JobName)
	if artifactName == "" {
		log.Printf("[NexusAgent] could not derive artifact name from job %q, skipping search", buildCtx.JobName)
		return result, nil
	}

	// Search for components matching the artifact name.
	found, err := a.searchComponents(ctx, baseURL, artifactName, auth)
	if err != nil {
		log.Printf("[NexusAgent] component search failed for %q: %v", artifactName, err)
		result.RepositoryStatus = "SEARCH_FAILED"
		return result, nil
	}

	if !found {
		result.ArtifactsAvailable = false
		result.MissingArtifacts = append(result.MissingArtifacts, artifactName)
		result.RepositoryStatus = "ARTIFACT_MISSING"
	}

	return result, nil
}

// searchComponents queries the Nexus search API for components matching the
// given artifact name.
func (a *NexusAgent) searchComponents(ctx context.Context, baseURL, artifactName, auth string) (bool, error) {
	url := fmt.Sprintf("%s/service/rest/v1/search?name=%s", baseURL, artifactName)
	authHeader := ""
	if auth != "" {
		authHeader = "Authorization"
	}
	body, err := doRequest(ctx, url, authHeader, auth)
	if err != nil {
		// A 404 means no results.
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, err
	}

	var resp nexusSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("parsing Nexus search response: %w", err)
	}

	return len(resp.Items) > 0, nil
}

// deriveArtifactName extracts the artifact name from a Jenkins job name by
// taking the last path segment. For example, "folder/subfolder/my-app" yields
// "my-app".
func (a *NexusAgent) deriveArtifactName(jobName string) string {
	if jobName == "" {
		return ""
	}
	parts := strings.Split(jobName, "/")
	return parts[len(parts)-1]
}

// --- internal JSON shapes ------------------------------------------------

type nexusSearchResponse struct {
	Items []nexusComponent `json:"items"`
}

type nexusComponent struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Name       string `json:"name"`
	Version    string `json:"version"`
}
