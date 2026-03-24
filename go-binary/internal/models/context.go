package models

// BuildContext captures all relevant information about a Jenkins build that is
// needed to perform root cause analysis.
type BuildContext struct {
	JobName             string            `json:"jobName"`
	BuildNumber         int               `json:"buildNumber"`
	BuildUrl            string            `json:"buildUrl"`
	JenkinsUrl          string            `json:"jenkinsUrl"`
	Repository          string            `json:"repository"`
	Branch              string            `json:"branch"`
	CommitHash          string            `json:"commitHash"`
	AgentName           string            `json:"agentName"`
	FailedStage         string            `json:"failedStage"`
	SuspectedRepository string            `json:"suspectedRepository"`
	ErrorMessages       []string          `json:"errorMessages"`
	FailedTests         []string          `json:"failedTests"`
	AllStages           []string          `json:"allStages"`
	StageRepoMapping    map[string]string `json:"stageRepoMapping"`
	ConsoleLog          string            `json:"consoleLog"`
}
