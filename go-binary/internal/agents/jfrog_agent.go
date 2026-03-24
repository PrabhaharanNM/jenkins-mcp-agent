package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// JFrogAgent queries the JFrog Artifactory REST API to check artifact
// availability and detect dependency resolution issues.
type JFrogAgent struct {
	req *models.AnalysisRequest
}

// NewJFrogAgent creates a JFrogAgent bound to the given request.
func NewJFrogAgent(req *models.AnalysisRequest) *JFrogAgent {
	return &JFrogAgent{req: req}
}

// Analyze checks artifact storage and scans error messages for dependency
// resolution failures. If JFrog configuration is not provided, it returns
// an empty result.
func (a *JFrogAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.JFrogAgentResult, error) {
	cfg := a.req.JFrog
	if cfg.Url == "" {
		log.Printf("[JFrogAgent] JFrog config not provided, skipping")
		return &models.JFrogAgentResult{}, nil
	}

	result := &models.JFrogAgentResult{
		ArtifactsAvailable: true,
		RepositoryStatus:   "OK",
	}

	baseURL := strings.TrimRight(cfg.Url, "/")
	authHeader, authValue := a.authHeaders()

	// Check for common dependency resolution errors in console log.
	depErrors := a.checkDependencyErrors(buildCtx.ErrorMessages)
	if len(depErrors) > 0 {
		result.MissingArtifacts = depErrors
		result.ArtifactsAvailable = false
		result.RepositoryStatus = "DEPENDENCY_RESOLUTION_FAILURE"
	}

	// Try to check artifact storage for the job if we can infer a repo path.
	repoPath := a.inferRepoPath(buildCtx)
	if repoPath != "" {
		available, err := a.checkArtifactExists(ctx, baseURL, repoPath, authHeader, authValue)
		if err != nil {
			log.Printf("[JFrogAgent] artifact check failed for %s: %v", repoPath, err)
		} else if !available {
			result.ArtifactsAvailable = false
			result.MissingArtifacts = append(result.MissingArtifacts, repoPath)
			if result.RepositoryStatus == "OK" {
				result.RepositoryStatus = "ARTIFACT_MISSING"
			}
		}
	}

	return result, nil
}

// checkArtifactExists queries the Artifactory storage API to verify whether
// an artifact path exists.
func (a *JFrogAgent) checkArtifactExists(ctx context.Context, baseURL, repoPath, authHeader, authValue string) (bool, error) {
	url := fmt.Sprintf("%s/api/storage/%s", baseURL, repoPath)
	body, err := doRequest(ctx, url, authHeader, authValue)
	if err != nil {
		// A 404 means the artifact doesn't exist.
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, err
	}

	// If we got a response, check that it contains expected storage info.
	var info struct {
		URI  string `json:"uri"`
		Repo string `json:"repo"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return false, fmt.Errorf("parsing storage response: %w", err)
	}
	return info.URI != "", nil
}

// checkDependencyErrors scans build error messages for common dependency
// resolution failure patterns.
func (a *JFrogAgent) checkDependencyErrors(errorMessages []string) []string {
	depPatterns := []string{
		"Could not resolve dependencies",
		"Could not find artifact",
		"Failed to collect dependencies",
		"Unable to resolve artifact",
		"Could not transfer artifact",
		"Artifact not found",
		"Resolution failed",
		"dependency resolution failed",
		"Downloaded from",
		"Non-resolvable parent POM",
	}

	var matches []string
	seen := make(map[string]bool)
	for _, msg := range errorMessages {
		for _, pat := range depPatterns {
			if strings.Contains(strings.ToLower(msg), strings.ToLower(pat)) {
				if !seen[msg] {
					seen[msg] = true
					matches = append(matches, msg)
				}
				break
			}
		}
	}
	return matches
}

// inferRepoPath tries to build an Artifactory storage path from the build
// context. This is best-effort and depends on naming conventions.
func (a *JFrogAgent) inferRepoPath(buildCtx *models.BuildContext) string {
	repo := buildCtx.SuspectedRepository
	if repo == "" {
		repo = buildCtx.JobName
	}
	if repo == "" {
		return ""
	}
	// Common convention: libs-release-local/<repo-name>/
	return fmt.Sprintf("libs-release-local/%s", repo)
}

// authHeaders returns the appropriate authentication header name and value
// for JFrog Artifactory. Uses API key header if only ApiKey is set, or
// basic auth if both Username and ApiKey are provided.
func (a *JFrogAgent) authHeaders() (string, string) {
	cfg := a.req.JFrog
	if cfg.Username != "" && cfg.ApiKey != "" {
		// Use basic auth with username + API key as password.
		return "Authorization", basicAuthValue(cfg.Username, cfg.ApiKey)
	}
	if cfg.ApiKey != "" {
		// API key only — use the dedicated JFrog header.
		return "X-JFrog-Art-Api", cfg.ApiKey
	}
	return "", ""
}
