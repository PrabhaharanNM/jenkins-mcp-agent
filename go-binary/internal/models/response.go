package models

// AnalysisResult is the final output of the root cause analysis, returned to the
// Jenkins plugin for display and downstream actions (Jira, email, etc.).
type AnalysisResult struct {
	Status           string   `json:"status"`
	Category         string   `json:"category"`
	RootCauseSummary string   `json:"rootCauseSummary"`
	RootCauseDetails string   `json:"rootCauseDetails"`
	ResponsibleTeam  string   `json:"responsibleTeam"`
	TeamEmail        string   `json:"teamEmail"`
	HtmlReport       string   `json:"htmlReport"`
	JiraTicketKey    string   `json:"jiraTicketKey"`
	JiraTicketUrl    string   `json:"jiraTicketUrl"`
	Evidence         []string `json:"evidence"`
	NextSteps        []string `json:"nextSteps"`
	ErrorMessage     string   `json:"errorMessage"`
	AnalysisTimeMs   int64    `json:"analysisTimeMs"`

	// ClaudeAnalysis contains the structured output from the Claude model.
	ClaudeAnalysis ClaudeAnalysis `json:"claudeAnalysis"`
}

// ClaudeAnalysis holds the structured analysis produced by Claude via Bedrock.
type ClaudeAnalysis struct {
	Category         string   `json:"category"`
	RootCauseSummary string   `json:"rootCauseSummary"`
	RootCauseDetails string   `json:"rootCauseDetails"`
	Evidence         []string `json:"evidence"`
	NextSteps        []string `json:"nextSteps"`
	Confidence       string   `json:"confidence"`
}
