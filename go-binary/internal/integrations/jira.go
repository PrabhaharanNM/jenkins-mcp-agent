package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// CreateJiraTicket creates a Jira issue for the failed build and returns the ticket key.
func CreateJiraTicket(req *models.AnalysisRequest, analysis *models.ClaudeAnalysis, teamMgr *models.TeamManager, buildCtx *models.BuildContext) (string, error) {
	if req.Jira.Url == "" || req.Jira.Project == "" {
		log.Println("[JIRA] Jira URL or project not configured, skipping ticket creation")
		return "", nil
	}

	jiraURL := strings.TrimRight(req.Jira.Url, "/")

	// Build summary (max 255 chars)
	summary := fmt.Sprintf("Build Failed: %s #%d - %s", buildCtx.JobName, buildCtx.BuildNumber, analysis.Category)
	if len(summary) > 255 {
		summary = summary[:255]
	}

	// Determine issue type and optional parent.
	// Use "Task" for Jira Cloud team-managed projects (more universally available than "Bug").
	issueTypeName := "Task"
	if req.Jira.EpicKey != "" {
		issueTypeName = "Sub-task"
	}

	// Build the request body
	fields := map[string]interface{}{
		"project":   map[string]string{"key": req.Jira.Project},
		"issuetype": map[string]string{"name": issueTypeName},
		"summary":   summary,
	}

	// Jira Cloud API v3 requires ADF format for description; v2 uses plain text.
	if strings.Contains(jiraURL, "atlassian.net") {
		fields["description"] = convertToADF(analysis, buildCtx)
	} else {
		fields["description"] = convertToJiraMarkup(analysis, buildCtx)
	}

	if req.Jira.EpicKey != "" {
		fields["parent"] = map[string]string{"key": req.Jira.EpicKey}
	}

	body := map[string]interface{}{
		"fields": fields,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		log.Printf("[JIRA] Failed to marshal request body: %v", err)
		return "", fmt.Errorf("failed to marshal jira request: %w", err)
	}

	// Use API v3 for Jira Cloud (atlassian.net), v2 for Server.
	apiVersion := "2"
	if strings.Contains(jiraURL, "atlassian.net") {
		apiVersion = "3"
	}
	apiURL := fmt.Sprintf("%s/rest/api/%s/issue", jiraURL, apiVersion)
	httpReq, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		log.Printf("[JIRA] Failed to create HTTP request: %v", err)
		return "", fmt.Errorf("failed to create jira http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(req.Jira.Username, req.Jira.ApiToken)

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[JIRA] HTTP request failed: %v", err)
		return "", fmt.Errorf("jira http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[JIRA] Failed to read response body: %v", err)
		return "", fmt.Errorf("failed to read jira response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[JIRA] API returned status %d: %s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("jira API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response for the issue key
	var result struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Printf("[JIRA] Failed to parse response: %v", err)
		return "", fmt.Errorf("failed to parse jira response: %w", err)
	}

	log.Printf("[JIRA] Created ticket: %s", result.Key)
	return result.Key, nil
}

// convertToJiraMarkup converts the Claude analysis into Jira wiki markup format.
func convertToJiraMarkup(analysis *models.ClaudeAnalysis, buildCtx *models.BuildContext) string {
	var sb strings.Builder

	// Root Cause Summary
	sb.WriteString("h2. Root Cause Summary\n")
	sb.WriteString(analysis.RootCauseSummary)
	sb.WriteString("\n\n")

	if analysis.RootCauseDetails != "" {
		sb.WriteString(analysis.RootCauseDetails)
		sb.WriteString("\n\n")
	}

	// Evidence
	if len(analysis.Evidence) > 0 {
		sb.WriteString("h2. Evidence\n")
		for _, e := range analysis.Evidence {
			sb.WriteString(fmt.Sprintf("* %s\n", e))
		}
		sb.WriteString("\n")
	}

	// Next Steps
	if len(analysis.NextSteps) > 0 {
		sb.WriteString("h2. Next Steps\n")
		for i, step := range analysis.NextSteps {
			sb.WriteString(fmt.Sprintf("# %d. %s\n", i+1, step))
		}
		sb.WriteString("\n")
	}

	// Build Info
	sb.WriteString("h2. Build Info\n")
	sb.WriteString(fmt.Sprintf("* *Job:* %s\n", buildCtx.JobName))
	sb.WriteString(fmt.Sprintf("* *Build:* #%d\n", buildCtx.BuildNumber))
	sb.WriteString(fmt.Sprintf("* *URL:* [%s|%s]\n", buildCtx.BuildUrl, buildCtx.BuildUrl))
	if buildCtx.FailedStage != "" {
		sb.WriteString(fmt.Sprintf("* *Failed Stage:* %s\n", buildCtx.FailedStage))
	}
	if buildCtx.Repository != "" {
		sb.WriteString(fmt.Sprintf("* *Repository:* %s\n", buildCtx.Repository))
	}
	if buildCtx.Branch != "" {
		sb.WriteString(fmt.Sprintf("* *Branch:* %s\n", buildCtx.Branch))
	}
	sb.WriteString(fmt.Sprintf("* *Confidence:* %s\n", analysis.Confidence))

	return sb.String()
}

// convertToADF converts the Claude analysis into Atlassian Document Format (ADF)
// for Jira Cloud API v3.
func convertToADF(analysis *models.ClaudeAnalysis, buildCtx *models.BuildContext) map[string]interface{} {
	content := []interface{}{}

	// Helper to create a text paragraph
	textParagraph := func(text string) map[string]interface{} {
		return map[string]interface{}{
			"type": "paragraph",
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": text},
			},
		}
	}

	// Heading
	heading := func(level int, text string) map[string]interface{} {
		return map[string]interface{}{
			"type":  "heading",
			"attrs": map[string]interface{}{"level": level},
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": text},
			},
		}
	}

	// Root Cause
	content = append(content, heading(2, "Root Cause Summary"))
	content = append(content, textParagraph(analysis.RootCauseSummary))
	if analysis.RootCauseDetails != "" {
		content = append(content, textParagraph(analysis.RootCauseDetails))
	}

	// Evidence
	if len(analysis.Evidence) > 0 {
		content = append(content, heading(2, "Evidence"))
		items := []interface{}{}
		for _, e := range analysis.Evidence {
			items = append(items, map[string]interface{}{
				"type": "listItem",
				"content": []interface{}{textParagraph(e)},
			})
		}
		content = append(content, map[string]interface{}{
			"type":    "bulletList",
			"content": items,
		})
	}

	// Next Steps
	if len(analysis.NextSteps) > 0 {
		content = append(content, heading(2, "Next Steps"))
		items := []interface{}{}
		for _, step := range analysis.NextSteps {
			items = append(items, map[string]interface{}{
				"type": "listItem",
				"content": []interface{}{textParagraph(step)},
			})
		}
		content = append(content, map[string]interface{}{
			"type":    "orderedList",
			"content": items,
		})
	}

	// Build Info
	content = append(content, heading(2, "Build Info"))
	content = append(content, textParagraph(fmt.Sprintf("Job: %s | Build: #%d | Branch: %s | Confidence: %s",
		buildCtx.JobName, buildCtx.BuildNumber, buildCtx.Branch, analysis.Confidence)))

	return map[string]interface{}{
		"version": 1,
		"type":    "doc",
		"content": content,
	}
}
