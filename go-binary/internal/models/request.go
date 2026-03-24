package models

// AnalysisRequest represents the incoming request from the Jenkins plugin to perform
// root cause analysis on a failed build.
type AnalysisRequest struct {
	AnalysisID  string `json:"analysisId"`
	JobName     string `json:"jobName"`
	BuildNumber int    `json:"buildNumber"`
	JenkinsUrl  string `json:"jenkinsUrl"`
	BuildUrl    string `json:"buildUrl"`

	// AWS / Bedrock configuration
	AWS AWSConfig `json:"aws"`

	// BitBucket configuration
	BitBucket BitBucketConfig `json:"bitbucket"`

	// Kubernetes configuration
	Kubernetes KubernetesConfig `json:"kubernetes"`

	// JFrog configuration
	JFrog JFrogConfig `json:"jfrog"`

	// Jira configuration
	Jira JiraConfig `json:"jira"`

	// Email configuration
	Email EmailConfig `json:"email"`

	// TeamMappings is a JSON string mapping repositories to team info.
	TeamMappings string `json:"teamMappings"`

	// DevopsManager is a JSON string with the devops manager contact info.
	DevopsManager string `json:"devopsManager"`

	// Software category selection
	Categories SoftwareCategories `json:"categories"`

	// Docker configuration (when clusterType is "docker")
	Docker DockerConfig `json:"docker"`

	// Nexus configuration (when artifactManager is "nexus")
	Nexus NexusConfig `json:"nexus"`

	// GitHub configuration (for Jenkins users with GitHub SCM)
	GitHub GitHubSCMConfig `json:"github"`

	// Repositories lists the repositories involved in the build pipeline.
	Repositories []RepositoryConfig `json:"repositories"`
}

// SoftwareCategories allows users to select which software stack to analyze.
// Empty values mean auto-detect (run agents based on config presence).
type SoftwareCategories struct {
	RepoSoftware    string `json:"repoSoftware"`    // "bitbucket", "github", or ""
	ClusterType     string `json:"clusterType"`      // "kubernetes", "docker", or ""
	ArtifactManager string `json:"artifactManager"`  // "jfrog", "nexus", or ""
}

// DockerConfig holds Docker daemon connection details.
type DockerConfig struct {
	Host      string `json:"host"`      // e.g., "unix:///var/run/docker.sock" or "tcp://host:2376"
	TlsCert   string `json:"tlsCert"`
	TlsKey    string `json:"tlsKey"`
	TlsCaCert string `json:"tlsCaCert"`
}

// NexusConfig holds Sonatype Nexus connection details.
type NexusConfig struct {
	Url      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// GitHubSCMConfig holds GitHub API connection details for Jenkins users.
type GitHubSCMConfig struct {
	Token  string `json:"token"`
	ApiUrl string `json:"apiUrl"`
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
}

// AWSConfig holds Claude AI provider configuration.
// Supports "bedrock" (default), "direct" (Anthropic API), or "max" (Claude Max/Teams).
type AWSConfig struct {
	Provider         string `json:"provider"`         // "bedrock" (default), "direct", "max"
	Region           string `json:"region"`           // AWS region for Bedrock
	ModelId          string `json:"modelId"`          // Model ID (Bedrock or Anthropic format)
	Profile          string `json:"profile"`          // AWS profile for Bedrock
	VpcEndpoint      string `json:"vpcEndpoint"`      // VPC endpoint for Bedrock
	AnthropicApiKey  string `json:"anthropicApiKey"`  // API key for direct/max provider
	AnthropicBaseUrl string `json:"anthropicBaseUrl"` // Base URL override (for max/proxy)
}

// BitBucketConfig holds BitBucket server connection details.
type BitBucketConfig struct {
	Url      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// KubernetesConfig holds Kubernetes API connection details.
type KubernetesConfig struct {
	ApiUrl    string `json:"apiUrl"`
	Token     string `json:"token"`
	Namespace string `json:"namespace"`
}

// JFrogConfig holds JFrog Artifactory connection details.
type JFrogConfig struct {
	Url      string `json:"url"`
	Username string `json:"username"`
	ApiKey   string `json:"apiKey"`
}

// JiraConfig holds Jira connection and project details.
type JiraConfig struct {
	Url             string `json:"url"`
	Username        string `json:"username"`
	ApiToken        string `json:"apiToken"`
	Project         string `json:"project"`
	EpicKey         string `json:"epicKey"`
	DevopsAssignee  string `json:"devopsAssignee"`
}

// EmailConfig holds SMTP email configuration.
type EmailConfig struct {
	SmtpHost    string `json:"smtpHost"`
	SmtpPort    int    `json:"smtpPort"`
	EnableSsl   bool   `json:"enableSsl"`
	FromAddress string `json:"fromAddress"`
	FromName    string `json:"fromName"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

// RepositoryConfig identifies a single source repository.
type RepositoryConfig struct {
	Name      string `json:"name"`
	Workspace string `json:"workspace"`
	Branch    string `json:"branch"`
}
