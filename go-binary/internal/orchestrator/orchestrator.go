package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/agents"
	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/claude"
	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/correlation"
	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/integrations"
	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/parser"
	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/reporting"
	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/storage"
	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/team"
)

// Analyze performs the full root cause analysis pipeline for a failed Jenkins build.
// It fetches logs, runs MCP agents in parallel, cross-correlates results, invokes
// Claude for AI analysis, assigns a responsible team, and produces an HTML report.
func Analyze(ctx context.Context, req *models.AnalysisRequest) (*models.AnalysisResult, error) {
	startTime := time.Now()

	// 1. Mark analysis as in-progress.
	if err := storage.SaveStatus(req.AnalysisID, "in-progress"); err != nil {
		log.Printf("warning: failed to save in-progress status: %v", err)
	}

	// 2. Fetch the Jenkins console log.
	consoleLog, err := fetchConsoleLog(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch console log: %w", err)
	}

	// 3. Parse the console log to extract build context.
	buildContext := parser.Parse(consoleLog)
	buildContext.JobName = req.JobName
	buildContext.BuildNumber = req.BuildNumber
	buildContext.BuildUrl = req.BuildUrl
	buildContext.JenkinsUrl = req.JenkinsUrl
	buildContext.ConsoleLog = consoleLog

	// 4. Run MCP agents in parallel based on software category selection.
	mcpResults := runAgentsParallel(ctx, req, buildContext)

	// 5. Cross-correlate results from all agents.
	correlationResult := correlation.Analyze(buildContext, mcpResults)

	// 6. Call Claude AI for structured analysis.
	claudeAnalysis, err := claude.Analyze(ctx, req, buildContext, mcpResults, correlationResult)
	if err != nil {
		return nil, fmt.Errorf("claude analysis failed: %w", err)
	}

	// 7. Assign responsible team.
	teamManager := team.Assign(req, buildContext, correlationResult)

	// 8. Generate HTML report.
	htmlReport := reporting.GenerateHTML(claudeAnalysis, buildContext, teamManager)

	// 9. Run integrations synchronously to ensure they complete before exit.
	jiraKey, jiraUrl := "", ""
	if ticketKey, err := integrations.CreateJiraTicket(req, claudeAnalysis, teamManager, buildContext); err != nil {
		log.Printf("warning: jira ticket creation failed: %v", err)
	} else if ticketKey != "" {
		jiraKey = ticketKey
		jiraUrl = req.Jira.Url + "/browse/" + ticketKey
	}
	if err := integrations.SendEmail(req, claudeAnalysis, teamManager, htmlReport, buildContext); err != nil {
		log.Printf("warning: email notification failed: %v", err)
	}
	if err := integrations.TrackMTTR(req, claudeAnalysis, teamManager, buildContext); err != nil {
		log.Printf("warning: MTTR tracking failed: %v", err)
	}

	// 10. Build the final result.
	result := &models.AnalysisResult{
		Status:           "completed",
		Category:         claudeAnalysis.Category,
		RootCauseSummary: claudeAnalysis.RootCauseSummary,
		RootCauseDetails: claudeAnalysis.RootCauseDetails,
		ResponsibleTeam:  teamManager.Name,
		TeamEmail:        teamManager.Email,
		HtmlReport:       htmlReport,
		Evidence:         claudeAnalysis.Evidence,
		NextSteps:        claudeAnalysis.NextSteps,
		ClaudeAnalysis:   *claudeAnalysis,
		AnalysisTimeMs:   time.Since(startTime).Milliseconds(),
		JiraTicketKey:    jiraKey,
		JiraTicketUrl:    jiraUrl,
	}

	// 11. Persist the final result.
	if err := storage.Save(req.AnalysisID, result); err != nil {
		log.Printf("warning: failed to save analysis result: %v", err)
	}

	// 12. POST to MCP Dashboard (synchronous to ensure it completes before exit).
	postToDashboard(req, result, buildContext, teamManager)

	return result, nil
}

// runAgentsParallel selects and runs MCP agents based on software category selection.
func runAgentsParallel(ctx context.Context, req *models.AnalysisRequest, buildCtx *models.BuildContext) *models.McpResults {
	results := &models.McpResults{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	run := func(name string, fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Printf("starting %s agent", name)
			fn()
		}()
	}

	// Jenkins agent always runs (it's the CI system itself).
	run("jenkins", func() {
		result, err := agents.NewJenkinsAgent(req).Analyze(ctx, buildCtx)
		if err != nil {
			log.Printf("warning: jenkins agent failed: %v", err)
		}
		if result != nil {
			mu.Lock()
			results.JenkinsResult = result
			mu.Unlock()
		}
	})

	// SCM agent selection based on categories.repoSoftware
	switch strings.ToLower(req.Categories.RepoSoftware) {
	case "bitbucket":
		run("bitbucket", func() {
			result, err := agents.NewBitBucketAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: bitbucket agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.BitBucketResult = result
				mu.Unlock()
			}
		})
	case "github":
		run("github", func() {
			result, err := agents.NewGithubAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: github agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.GithubResult = result
				mu.Unlock()
			}
		})
	default:
		// Auto: run whichever has config populated
		if req.BitBucket.Url != "" {
			run("bitbucket", func() {
				result, err := agents.NewBitBucketAgent(req).Analyze(ctx, buildCtx)
				if err != nil {
					log.Printf("warning: bitbucket agent failed: %v", err)
				}
				if result != nil {
					mu.Lock()
					results.BitBucketResult = result
					mu.Unlock()
				}
			})
		}
		if req.GitHub.Token != "" {
			run("github", func() {
				result, err := agents.NewGithubAgent(req).Analyze(ctx, buildCtx)
				if err != nil {
					log.Printf("warning: github agent failed: %v", err)
				}
				if result != nil {
					mu.Lock()
					results.GithubResult = result
					mu.Unlock()
				}
			})
		}
	}

	// Cluster agent selection based on categories.clusterType
	switch strings.ToLower(req.Categories.ClusterType) {
	case "kubernetes":
		run("kubernetes", func() {
			result, err := agents.NewKubernetesAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: kubernetes agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.KubernetesResult = result
				mu.Unlock()
			}
		})
	case "docker":
		run("docker", func() {
			result, err := agents.NewDockerAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: docker agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.DockerResult = result
				mu.Unlock()
			}
		})
	default:
		if req.Kubernetes.ApiUrl != "" {
			run("kubernetes", func() {
				result, err := agents.NewKubernetesAgent(req).Analyze(ctx, buildCtx)
				if err != nil {
					log.Printf("warning: kubernetes agent failed: %v", err)
				}
				if result != nil {
					mu.Lock()
					results.KubernetesResult = result
					mu.Unlock()
				}
			})
		}
		if req.Docker.Host != "" {
			run("docker", func() {
				result, err := agents.NewDockerAgent(req).Analyze(ctx, buildCtx)
				if err != nil {
					log.Printf("warning: docker agent failed: %v", err)
				}
				if result != nil {
					mu.Lock()
					results.DockerResult = result
					mu.Unlock()
				}
			})
		}
	}

	// Artifact manager selection based on categories.artifactManager
	switch strings.ToLower(req.Categories.ArtifactManager) {
	case "jfrog":
		run("jfrog", func() {
			result, err := agents.NewJFrogAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: jfrog agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.JFrogResult = result
				mu.Unlock()
			}
		})
	case "nexus":
		run("nexus", func() {
			result, err := agents.NewNexusAgent(req).Analyze(ctx, buildCtx)
			if err != nil {
				log.Printf("warning: nexus agent failed: %v", err)
			}
			if result != nil {
				mu.Lock()
				results.NexusResult = result
				mu.Unlock()
			}
		})
	default:
		if req.JFrog.Url != "" {
			run("jfrog", func() {
				result, err := agents.NewJFrogAgent(req).Analyze(ctx, buildCtx)
				if err != nil {
					log.Printf("warning: jfrog agent failed: %v", err)
				}
				if result != nil {
					mu.Lock()
					results.JFrogResult = result
					mu.Unlock()
				}
			})
		}
		if req.Nexus.Url != "" {
			run("nexus", func() {
				result, err := agents.NewNexusAgent(req).Analyze(ctx, buildCtx)
				if err != nil {
					log.Printf("warning: nexus agent failed: %v", err)
				}
				if result != nil {
					mu.Lock()
					results.NexusResult = result
					mu.Unlock()
				}
			})
		}
	}

	wg.Wait()
	return results
}

// fetchConsoleLog retrieves the console output text for a Jenkins build using
// basic authentication and a 30-second timeout.
func fetchConsoleLog(ctx context.Context, req *models.AnalysisRequest) (string, error) {
	url := fmt.Sprintf("%s/job/%s/%d/consoleText", req.JenkinsUrl, req.JobName, req.BuildNumber)

	httpCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(httpCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	// Use environment-provided Jenkins credentials if available; otherwise
	// rely on anonymous read access (which must be enabled in Jenkins config).
	if user := os.Getenv("JENKINS_USER"); user != "" {
		httpReq.SetBasicAuth(user, os.Getenv("JENKINS_TOKEN"))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP GET %s returned status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	return string(body), nil
}

// postToDashboard sends analysis results to the MCP Dashboard (if MCP_DASHBOARD_URL is set).
func postToDashboard(req *models.AnalysisRequest, result *models.AnalysisResult, buildCtx *models.BuildContext, teamMgr *models.TeamManager) {
	dashURL := os.Getenv("MCP_DASHBOARD_URL")
	if dashURL == "" {
		return
	}

	payload := map[string]interface{}{
		"analysisId":       req.AnalysisID,
		"jobName":          req.JobName,
		"buildNumber":      req.BuildNumber,
		"buildUrl":         req.BuildUrl,
		"repository":       buildCtx.Repository,
		"branch":           buildCtx.Branch,
		"commitHash":       buildCtx.CommitHash,
		"failedStage":      buildCtx.FailedStage,
		"status":           result.Status,
		"category":         result.Category,
		"rootCauseSummary": result.RootCauseSummary,
		"rootCauseDetails": result.RootCauseDetails,
		"responsibleTeam":  result.ResponsibleTeam,
		"teamEmail":        result.TeamEmail,
		"confidence":       result.ClaudeAnalysis.Confidence,
		"evidence":         result.Evidence,
		"nextSteps":        result.NextSteps,
		"errorMessages":    buildCtx.ErrorMessages,
		"analysisTimeMs":   result.AnalysisTimeMs,
		"jiraTicketKey":    result.JiraTicketKey,
		"jiraTicketUrl":    result.JiraTicketUrl,
		"developer":        teamMgr.Name,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("warning: dashboard payload marshal: %v", err)
		return
	}

	url := strings.TrimRight(dashURL, "/") + "/api/ingest/jenkins"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("warning: dashboard POST failed: %v", err)
		return
	}
	resp.Body.Close()
	log.Printf("dashboard: posted to %s (status %d)", url, resp.StatusCode)
}
