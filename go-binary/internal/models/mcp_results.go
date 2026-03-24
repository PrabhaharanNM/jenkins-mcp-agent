package models

// McpResults aggregates the results collected from all MCP agents.
type McpResults struct {
	JenkinsResult    *JenkinsAgentResult    `json:"jenkinsResult"`
	BitBucketResult  *BitBucketAgentResult  `json:"bitBucketResult"`
	GithubResult     *GithubAgentResult     `json:"githubResult"`
	KubernetesResult *KubernetesAgentResult `json:"kubernetesResult"`
	DockerResult     *DockerAgentResult     `json:"dockerResult"`
	JFrogResult      *JFrogAgentResult      `json:"jfrogResult"`
	NexusResult      *NexusAgentResult      `json:"nexusResult"`
}

// JenkinsAgentResult holds data retrieved from the Jenkins MCP agent.
type JenkinsAgentResult struct {
	Stages         []StageInfo `json:"stages"`
	AgentOnline    bool        `json:"agentOnline"`
	AgentDiskFree  string      `json:"agentDiskFree"`
	ErrorArtifacts []string    `json:"errorArtifacts"`
}

// StageInfo describes a single pipeline stage.
type StageInfo struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs"`
}

// BitBucketAgentResult holds data retrieved from the BitBucket MCP agent.
type BitBucketAgentResult struct {
	CodeOwners    string       `json:"codeOwners"`
	RecentCommits []CommitInfo `json:"recentCommits"`
	ChangedFiles  []string     `json:"changedFiles"`
}

// CommitInfo describes a single source control commit.
type CommitInfo struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Message string `json:"message"`
	Date    string `json:"date"`
}

// KubernetesAgentResult holds data retrieved from the Kubernetes MCP agent.
type KubernetesAgentResult struct {
	PodStatuses  []PodStatus `json:"podStatuses"`
	OOMKills     []string    `json:"oomKills"`
	NodePressure bool        `json:"nodePressure"`
	Events       []string    `json:"events"`
}

// PodStatus describes the status of a single Kubernetes pod.
type PodStatus struct {
	Name         string `json:"name"`
	Phase        string `json:"phase"`
	Reason       string `json:"reason"`
	RestartCount int    `json:"restartCount"`
}

// JFrogAgentResult holds data retrieved from the JFrog MCP agent.
type JFrogAgentResult struct {
	ArtifactsAvailable bool     `json:"artifactsAvailable"`
	MissingArtifacts   []string `json:"missingArtifacts"`
	RepositoryStatus   string   `json:"repositoryStatus"`
}

// Correlation captures the cross-agent analysis that identifies the root cause
// category and the responsible team/repository.
type Correlation struct {
	RootCauseType         string   `json:"rootCauseType"`
	IsInfrastructure      bool     `json:"isInfrastructure"`
	ResponsibleRepository string   `json:"responsibleRepository"`
	ResponsibleTeam       string   `json:"responsibleTeam"`
	Evidence              []string `json:"evidence"`
}

// GithubAgentResult holds data retrieved from the GitHub API (for Jenkins users with GitHub SCM).
type GithubAgentResult struct {
	RecentCommits []CommitInfo `json:"recentCommits"`
	ChangedFiles  []string     `json:"changedFiles"`
	CodeOwners    string       `json:"codeOwners"`
	PrTitle       string       `json:"prTitle"`
	PrBody        string       `json:"prBody"`
}

// DockerAgentResult holds data retrieved from the Docker Engine API.
type DockerAgentResult struct {
	ContainerStatuses []ContainerStatus `json:"containerStatuses"`
	FailedContainers  []string          `json:"failedContainers"`
	OOMKilled         []string          `json:"oomKilled"`
	ImageIssues       []string          `json:"imageIssues"`
	DiskUsage         string            `json:"diskUsage"`
}

// ContainerStatus describes the status of a Docker container.
type ContainerStatus struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	State    string `json:"state"`
	Status   string `json:"status"`
	ExitCode int    `json:"exitCode"`
}

// NexusAgentResult holds data retrieved from Sonatype Nexus.
type NexusAgentResult struct {
	ArtifactsAvailable bool     `json:"artifactsAvailable"`
	MissingArtifacts   []string `json:"missingArtifacts"`
	RepositoryStatus   string   `json:"repositoryStatus"`
}

// TeamManager holds contact information for a team manager.
type TeamManager struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	JiraUsername string `json:"jiraUsername"`
}
