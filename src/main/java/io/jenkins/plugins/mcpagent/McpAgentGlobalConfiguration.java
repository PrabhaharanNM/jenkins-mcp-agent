package io.jenkins.plugins.mcpagent;

import hudson.Extension;
import hudson.util.Secret;
import jenkins.model.GlobalConfiguration;
import jenkins.model.Jenkins;
import net.sf.json.JSONObject;
import hudson.util.ListBoxModel;
import org.kohsuke.stapler.DataBoundSetter;
import org.kohsuke.stapler.StaplerRequest;

/**
 * Global configuration for the MCP Agent plugin.
 * Accessible from Manage Jenkins -> Configure System.
 */
@Extension
public class McpAgentGlobalConfiguration extends GlobalConfiguration {

    // Claude AI Provider
    private String awsProvider = "direct";
    private Secret anthropicApiKey;
    private String anthropicBaseUrl = "";

    // AWS Bedrock
    private String awsRegion = "us-west-2";
    private String awsModelId = "anthropic.claude-3-5-sonnet-20241022-v2:0";
    private String awsProfile = "";
    private String awsVpcEndpoint = "";

    // Jenkins
    private String jenkinsUrl = "";

    // BitBucket
    private String bitbucketUrl = "";
    private String bitbucketUsername = "";
    private Secret bitbucketPassword;

    // Kubernetes
    private String k8sApiUrl = "";
    private Secret k8sToken;
    private String k8sNamespace = "default";

    // JFrog
    private String jfrogUrl = "";
    private String jfrogUsername = "";
    private Secret jfrogApiKey;

    // Jira
    private String jiraUrl = "";
    private String jiraUsername = "";
    private Secret jiraApiToken;
    private String jiraProject = "";
    private String jiraEpicKey = "";
    private String jiraDevopsAssignee = "";

    // Email/SMTP
    private String smtpHost = "";
    private int smtpPort = 587;
    private boolean smtpSslEnabled = true;
    private String smtpFromAddress = "";
    private String smtpFromName = "MCP Build Failure Agent";
    private String smtpUsername = "";
    private Secret smtpPassword;

    // Team mappings (JSON strings)
    private String teamMappings = "{}";
    private String devopsManager = "{}";

    // Software Categories
    private String repoSoftware = "";
    private String clusterType = "";
    private String artifactManager = "";

    // Docker
    private String dockerHost = "";
    private Secret dockerTlsCert;
    private Secret dockerTlsKey;
    private Secret dockerTlsCaCert;

    // Nexus
    private String nexusUrl = "";
    private String nexusUsername = "";
    private Secret nexusPassword;

    // GitHub SCM (for Jenkins users with GitHub repos)
    private Secret githubToken;
    private String githubApiUrl = "https://api.github.com";
    private String githubOwner = "";
    private String githubRepo = "";

    public McpAgentGlobalConfiguration() {
        load();
        if (jenkinsUrl == null || jenkinsUrl.isEmpty()) {
            Jenkins jenkins = Jenkins.getInstanceOrNull();
            if (jenkins != null) {
                jenkinsUrl = jenkins.getRootUrl();
            }
        }
    }

    public static McpAgentGlobalConfiguration get() {
        return GlobalConfiguration.all().get(McpAgentGlobalConfiguration.class);
    }

    @Override
    public String getDisplayName() {
        return "MCP Agent: AI-Powered Build Failure Analysis";
    }

    @Override
    public boolean configure(StaplerRequest req, JSONObject json) throws FormException {
        req.bindJSON(this, json);
        save();
        return true;
    }

    // --- Claude AI Provider ---
    public String getAwsProvider() { return awsProvider; }
    @DataBoundSetter public void setAwsProvider(String v) { this.awsProvider = v; }

    public String getAnthropicApiKey() { return anthropicApiKey != null ? anthropicApiKey.getPlainText() : ""; }
    @DataBoundSetter public void setAnthropicApiKey(String v) { this.anthropicApiKey = Secret.fromString(v); }

    public String getAnthropicBaseUrl() { return anthropicBaseUrl; }
    @DataBoundSetter public void setAnthropicBaseUrl(String v) { this.anthropicBaseUrl = v; }

    // --- AWS Bedrock ---
    public String getAwsRegion() { return awsRegion; }
    @DataBoundSetter public void setAwsRegion(String awsRegion) { this.awsRegion = awsRegion; }

    public String getAwsModelId() { return awsModelId; }
    @DataBoundSetter public void setAwsModelId(String awsModelId) { this.awsModelId = awsModelId; }

    public String getAwsProfile() { return awsProfile; }
    @DataBoundSetter public void setAwsProfile(String awsProfile) { this.awsProfile = awsProfile; }

    public String getAwsVpcEndpoint() { return awsVpcEndpoint; }
    @DataBoundSetter public void setAwsVpcEndpoint(String awsVpcEndpoint) { this.awsVpcEndpoint = awsVpcEndpoint; }

    // --- Jenkins ---
    public String getJenkinsUrl() { return jenkinsUrl; }
    @DataBoundSetter public void setJenkinsUrl(String jenkinsUrl) { this.jenkinsUrl = jenkinsUrl; }

    // --- BitBucket ---
    public String getBitbucketUrl() { return bitbucketUrl; }
    @DataBoundSetter public void setBitbucketUrl(String bitbucketUrl) { this.bitbucketUrl = bitbucketUrl; }

    public String getBitbucketUsername() { return bitbucketUsername; }
    @DataBoundSetter public void setBitbucketUsername(String bitbucketUsername) { this.bitbucketUsername = bitbucketUsername; }

    public String getBitbucketPassword() { return bitbucketPassword != null ? bitbucketPassword.getPlainText() : ""; }
    @DataBoundSetter public void setBitbucketPassword(String password) { this.bitbucketPassword = Secret.fromString(password); }

    // --- Kubernetes ---
    public String getK8sApiUrl() { return k8sApiUrl; }
    @DataBoundSetter public void setK8sApiUrl(String k8sApiUrl) { this.k8sApiUrl = k8sApiUrl; }

    public String getK8sToken() { return k8sToken != null ? k8sToken.getPlainText() : ""; }
    @DataBoundSetter public void setK8sToken(String token) { this.k8sToken = Secret.fromString(token); }

    public String getK8sNamespace() { return k8sNamespace; }
    @DataBoundSetter public void setK8sNamespace(String k8sNamespace) { this.k8sNamespace = k8sNamespace; }

    // --- JFrog ---
    public String getJfrogUrl() { return jfrogUrl; }
    @DataBoundSetter public void setJfrogUrl(String jfrogUrl) { this.jfrogUrl = jfrogUrl; }

    public String getJfrogUsername() { return jfrogUsername; }
    @DataBoundSetter public void setJfrogUsername(String jfrogUsername) { this.jfrogUsername = jfrogUsername; }

    public String getJfrogApiKey() { return jfrogApiKey != null ? jfrogApiKey.getPlainText() : ""; }
    @DataBoundSetter public void setJfrogApiKey(String apiKey) { this.jfrogApiKey = Secret.fromString(apiKey); }

    // --- Jira ---
    public String getJiraUrl() { return jiraUrl; }
    @DataBoundSetter public void setJiraUrl(String jiraUrl) { this.jiraUrl = jiraUrl; }

    public String getJiraUsername() { return jiraUsername; }
    @DataBoundSetter public void setJiraUsername(String jiraUsername) { this.jiraUsername = jiraUsername; }

    public String getJiraApiToken() { return jiraApiToken != null ? jiraApiToken.getPlainText() : ""; }
    @DataBoundSetter public void setJiraApiToken(String token) { this.jiraApiToken = Secret.fromString(token); }

    public String getJiraProject() { return jiraProject; }
    @DataBoundSetter public void setJiraProject(String jiraProject) { this.jiraProject = jiraProject; }

    public String getJiraEpicKey() { return jiraEpicKey; }
    @DataBoundSetter public void setJiraEpicKey(String jiraEpicKey) { this.jiraEpicKey = jiraEpicKey; }

    public String getJiraDevopsAssignee() { return jiraDevopsAssignee; }
    @DataBoundSetter public void setJiraDevopsAssignee(String jiraDevopsAssignee) { this.jiraDevopsAssignee = jiraDevopsAssignee; }

    // --- Email/SMTP ---
    public String getSmtpHost() { return smtpHost; }
    @DataBoundSetter public void setSmtpHost(String smtpHost) { this.smtpHost = smtpHost; }

    public int getSmtpPort() { return smtpPort; }
    @DataBoundSetter public void setSmtpPort(int smtpPort) { this.smtpPort = smtpPort; }

    public boolean isSmtpSslEnabled() { return smtpSslEnabled; }
    @DataBoundSetter public void setSmtpSslEnabled(boolean smtpSslEnabled) { this.smtpSslEnabled = smtpSslEnabled; }

    public String getSmtpFromAddress() { return smtpFromAddress; }
    @DataBoundSetter public void setSmtpFromAddress(String smtpFromAddress) { this.smtpFromAddress = smtpFromAddress; }

    public String getSmtpFromName() { return smtpFromName; }
    @DataBoundSetter public void setSmtpFromName(String smtpFromName) { this.smtpFromName = smtpFromName; }

    public String getSmtpUsername() { return smtpUsername; }
    @DataBoundSetter public void setSmtpUsername(String smtpUsername) { this.smtpUsername = smtpUsername; }

    public String getSmtpPassword() { return smtpPassword != null ? smtpPassword.getPlainText() : ""; }
    @DataBoundSetter public void setSmtpPassword(String password) { this.smtpPassword = Secret.fromString(password); }

    // --- Team Mappings ---
    public String getTeamMappings() { return teamMappings; }
    @DataBoundSetter public void setTeamMappings(String teamMappings) { this.teamMappings = teamMappings; }

    public String getDevopsManager() { return devopsManager; }
    @DataBoundSetter public void setDevopsManager(String devopsManager) { this.devopsManager = devopsManager; }

    // --- Software Categories ---
    public String getRepoSoftware() { return repoSoftware; }
    @DataBoundSetter public void setRepoSoftware(String v) { this.repoSoftware = v; }

    public String getClusterType() { return clusterType; }
    @DataBoundSetter public void setClusterType(String v) { this.clusterType = v; }

    public String getArtifactManager() { return artifactManager; }
    @DataBoundSetter public void setArtifactManager(String v) { this.artifactManager = v; }

    // --- Docker ---
    public String getDockerHost() { return dockerHost; }
    @DataBoundSetter public void setDockerHost(String v) { this.dockerHost = v; }

    public String getDockerTlsCert() { return dockerTlsCert != null ? dockerTlsCert.getPlainText() : ""; }
    @DataBoundSetter public void setDockerTlsCert(String v) { this.dockerTlsCert = Secret.fromString(v); }

    public String getDockerTlsKey() { return dockerTlsKey != null ? dockerTlsKey.getPlainText() : ""; }
    @DataBoundSetter public void setDockerTlsKey(String v) { this.dockerTlsKey = Secret.fromString(v); }

    public String getDockerTlsCaCert() { return dockerTlsCaCert != null ? dockerTlsCaCert.getPlainText() : ""; }
    @DataBoundSetter public void setDockerTlsCaCert(String v) { this.dockerTlsCaCert = Secret.fromString(v); }

    // --- Nexus ---
    public String getNexusUrl() { return nexusUrl; }
    @DataBoundSetter public void setNexusUrl(String v) { this.nexusUrl = v; }

    public String getNexusUsername() { return nexusUsername; }
    @DataBoundSetter public void setNexusUsername(String v) { this.nexusUsername = v; }

    public String getNexusPassword() { return nexusPassword != null ? nexusPassword.getPlainText() : ""; }
    @DataBoundSetter public void setNexusPassword(String v) { this.nexusPassword = Secret.fromString(v); }

    // --- GitHub SCM ---
    public String getGithubToken() { return githubToken != null ? githubToken.getPlainText() : ""; }
    @DataBoundSetter public void setGithubToken(String v) { this.githubToken = Secret.fromString(v); }

    public String getGithubApiUrl() { return githubApiUrl; }
    @DataBoundSetter public void setGithubApiUrl(String v) { this.githubApiUrl = v; }

    public String getGithubOwner() { return githubOwner; }
    @DataBoundSetter public void setGithubOwner(String v) { this.githubOwner = v; }

    public String getGithubRepo() { return githubRepo; }
    @DataBoundSetter public void setGithubRepo(String v) { this.githubRepo = v; }

    // --- ListBoxModel fill methods ---
    public ListBoxModel doFillAwsProviderItems() {
        ListBoxModel items = new ListBoxModel();
        items.add("AWS Bedrock", "bedrock");
        items.add("Anthropic Direct API", "direct");
        items.add("Claude Max / Teams", "max");
        return items;
    }

    public ListBoxModel doFillRepoSoftwareItems() {
        ListBoxModel items = new ListBoxModel();
        items.add("Auto (run all configured)", "");
        items.add("BitBucket", "bitbucket");
        items.add("GitHub", "github");
        return items;
    }

    public ListBoxModel doFillClusterTypeItems() {
        ListBoxModel items = new ListBoxModel();
        items.add("Auto (run all configured)", "");
        items.add("Kubernetes", "kubernetes");
        items.add("Docker", "docker");
        return items;
    }

    public ListBoxModel doFillArtifactManagerItems() {
        ListBoxModel items = new ListBoxModel();
        items.add("Auto (run all configured)", "");
        items.add("JFrog Artifactory", "jfrog");
        items.add("Nexus Repository", "nexus");
        return items;
    }
}
