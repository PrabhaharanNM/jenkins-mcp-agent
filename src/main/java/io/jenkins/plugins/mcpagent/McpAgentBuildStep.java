package io.jenkins.plugins.mcpagent;

import com.google.gson.Gson;
import com.google.gson.JsonObject;
import hudson.Extension;
import hudson.Launcher;
import hudson.model.AbstractBuild;
import hudson.model.AbstractProject;
import hudson.model.BuildListener;
import hudson.model.Result;
import hudson.model.AbstractDescribableImpl;
import hudson.model.Descriptor;
import hudson.tasks.BuildStepDescriptor;
import hudson.tasks.BuildStepMonitor;
import hudson.tasks.Notifier;
import hudson.tasks.Publisher;
import org.jenkinsci.Symbol;
import org.kohsuke.stapler.DataBoundConstructor;
import org.kohsuke.stapler.DataBoundSetter;

import java.io.PrintStream;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

/**
 * Post-build action that triggers AI-powered failure analysis.
 * Calls the Go binary as a subprocess to perform the heavy processing.
 */
public class McpAgentBuildStep extends Notifier {

    private List<RepositoryConfig> repositories = new ArrayList<>();

    @DataBoundConstructor
    public McpAgentBuildStep() {
    }

    public List<RepositoryConfig> getRepositories() {
        return repositories;
    }

    @DataBoundSetter
    public void setRepositories(List<RepositoryConfig> repositories) {
        this.repositories = repositories != null ? repositories : new ArrayList<>();
    }

    @Override
    public BuildStepMonitor getRequiredMonitorService() {
        return BuildStepMonitor.NONE;
    }

    @Override
    public boolean perform(AbstractBuild<?, ?> build, Launcher launcher, BuildListener listener) {
        PrintStream logger = listener.getLogger();
        Result result = build.getResult();

        // Only trigger on failure or unstable builds
        if (result == null || result.isBetterThan(Result.UNSTABLE)) {
            logger.println("[MCP Agent] Build succeeded - skipping analysis.");
            return true;
        }

        logger.println("[MCP Agent] Build failure detected. Starting AI-powered analysis...");

        try {
            String analysisId = UUID.randomUUID().toString();
            McpAgentGlobalConfiguration config = McpAgentGlobalConfiguration.get();

            // Build JSON request for the Go binary
            JsonObject request = buildRequest(build, analysisId, config);
            String requestJson = new Gson().toJson(request);

            logger.println("[MCP Agent] Analysis ID: " + analysisId);

            // Execute Go binary asynchronously
            String output = GoProcessExecutor.execute(requestJson);
            logger.println("[MCP Agent] " + output);

            // Build the results URL
            String buildUrl = build.getAbsoluteUrl();
            String resultsUrl = buildUrl + "mcp-analysis";
            logger.println("[MCP Agent] View results at: " + resultsUrl);

            // Add sidebar action to the build
            build.addAction(new McpAnalysisAction(analysisId, build));

        } catch (Exception e) {
            logger.println("[MCP Agent] Error starting analysis: " + e.getMessage());
            // Don't fail the build due to analysis errors
        }

        return true; // Never fail the build because of the plugin
    }

    private JsonObject buildRequest(AbstractBuild<?, ?> build, String analysisId,
                                     McpAgentGlobalConfiguration config) {
        JsonObject request = new JsonObject();
        request.addProperty("analysisId", analysisId);
        request.addProperty("jobName", build.getProject().getFullName());
        request.addProperty("buildNumber", build.getNumber());
        request.addProperty("jenkinsUrl", config.getJenkinsUrl());
        request.addProperty("buildUrl", build.getAbsoluteUrl());

        // Claude AI / AWS Bedrock settings
        JsonObject aws = new JsonObject();
        aws.addProperty("provider", config.getAwsProvider());
        aws.addProperty("region", config.getAwsRegion());
        aws.addProperty("modelId", config.getAwsModelId());
        aws.addProperty("profile", config.getAwsProfile());
        aws.addProperty("vpcEndpoint", config.getAwsVpcEndpoint());
        aws.addProperty("anthropicApiKey", config.getAnthropicApiKey());
        aws.addProperty("anthropicBaseUrl", config.getAnthropicBaseUrl());
        request.add("aws", aws);

        // BitBucket settings
        JsonObject bitbucket = new JsonObject();
        bitbucket.addProperty("url", config.getBitbucketUrl());
        bitbucket.addProperty("username", config.getBitbucketUsername());
        bitbucket.addProperty("password", config.getBitbucketPassword());
        request.add("bitbucket", bitbucket);

        // Kubernetes settings
        JsonObject kubernetes = new JsonObject();
        kubernetes.addProperty("apiUrl", config.getK8sApiUrl());
        kubernetes.addProperty("token", config.getK8sToken());
        kubernetes.addProperty("namespace", config.getK8sNamespace());
        request.add("kubernetes", kubernetes);

        // JFrog settings
        JsonObject jfrog = new JsonObject();
        jfrog.addProperty("url", config.getJfrogUrl());
        jfrog.addProperty("username", config.getJfrogUsername());
        jfrog.addProperty("apiKey", config.getJfrogApiKey());
        request.add("jfrog", jfrog);

        // Jira settings
        JsonObject jira = new JsonObject();
        jira.addProperty("url", config.getJiraUrl());
        jira.addProperty("username", config.getJiraUsername());
        jira.addProperty("apiToken", config.getJiraApiToken());
        jira.addProperty("project", config.getJiraProject());
        jira.addProperty("epicKey", config.getJiraEpicKey());
        jira.addProperty("devopsAssignee", config.getJiraDevopsAssignee());
        request.add("jira", jira);

        // Email settings
        JsonObject email = new JsonObject();
        email.addProperty("smtpHost", config.getSmtpHost());
        email.addProperty("smtpPort", config.getSmtpPort());
        email.addProperty("enableSsl", config.isSmtpSslEnabled());
        email.addProperty("fromAddress", config.getSmtpFromAddress());
        email.addProperty("fromName", config.getSmtpFromName());
        email.addProperty("username", config.getSmtpUsername());
        email.addProperty("password", config.getSmtpPassword());
        request.add("email", email);

        // Team mappings
        request.addProperty("teamMappings", config.getTeamMappings());
        request.addProperty("devopsManager", config.getDevopsManager());

        // Software categories
        JsonObject categories = new JsonObject();
        categories.addProperty("repoSoftware", config.getRepoSoftware());
        categories.addProperty("clusterType", config.getClusterType());
        categories.addProperty("artifactManager", config.getArtifactManager());
        request.add("categories", categories);

        // Docker settings
        JsonObject docker = new JsonObject();
        docker.addProperty("host", config.getDockerHost());
        docker.addProperty("tlsCert", config.getDockerTlsCert());
        docker.addProperty("tlsKey", config.getDockerTlsKey());
        docker.addProperty("tlsCaCert", config.getDockerTlsCaCert());
        request.add("docker", docker);

        // Nexus settings
        JsonObject nexus = new JsonObject();
        nexus.addProperty("url", config.getNexusUrl());
        nexus.addProperty("username", config.getNexusUsername());
        nexus.addProperty("password", config.getNexusPassword());
        request.add("nexus", nexus);

        // GitHub SCM settings
        JsonObject github = new JsonObject();
        github.addProperty("token", config.getGithubToken());
        github.addProperty("apiUrl", config.getGithubApiUrl());
        github.addProperty("owner", config.getGithubOwner());
        github.addProperty("repo", config.getGithubRepo());
        request.add("github", github);

        // Repository list
        Gson gson = new Gson();
        if (!repositories.isEmpty()) {
            request.add("repositories", gson.toJsonTree(repositories));
        }

        return request;
    }

    @Symbol("mcpAnalyze")
    @Extension
    public static final class DescriptorImpl extends BuildStepDescriptor<Publisher> {

        @Override
        public String getDisplayName() {
            return "MCP Agent: AI-Powered Failure Analysis";
        }

        @Override
        public boolean isApplicable(Class<? extends AbstractProject> jobType) {
            return true;
        }
    }

    /** Represents a repository configuration for analysis. */
    public static class RepositoryConfig extends AbstractDescribableImpl<RepositoryConfig> {
        private String name;
        private String workspace;
        private String branch;

        @DataBoundConstructor
        public RepositoryConfig(String name, String workspace, String branch) {
            this.name = name;
            this.workspace = workspace;
            this.branch = branch;
        }

        public String getName() { return name; }
        public String getWorkspace() { return workspace; }
        public String getBranch() { return branch; }

        @Extension
        public static class DescriptorImpl extends Descriptor<RepositoryConfig> {
            @Override
            public String getDisplayName() {
                return "Repository Configuration";
            }
        }
    }
}
