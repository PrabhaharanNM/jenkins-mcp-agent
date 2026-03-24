package io.jenkins.plugins.mcpagent;

import com.google.gson.Gson;
import com.google.gson.JsonObject;
import hudson.model.AbstractBuild;
import hudson.model.Action;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * Adds a sidebar link ("AI Analysis") to build pages.
 * Reads analysis results from the JSON file written by the Go binary.
 */
public class McpAnalysisAction implements Action {

    private static final Logger LOGGER = Logger.getLogger(McpAnalysisAction.class.getName());
    private static final String RESULTS_DIR = System.getProperty("java.io.tmpdir") + "/mcp-results";

    private final String analysisId;
    private final String buildDisplayName;

    public McpAnalysisAction(String analysisId, AbstractBuild<?, ?> build) {
        this.analysisId = analysisId;
        this.buildDisplayName = build.getFullDisplayName();
    }

    @Override
    public String getIconFileName() {
        return "document.png";
    }

    @Override
    public String getDisplayName() {
        return "AI Analysis";
    }

    @Override
    public String getUrlName() {
        return "mcp-analysis";
    }

    public String getAnalysisId() {
        return analysisId;
    }

    public String getBuildDisplayName() {
        return buildDisplayName;
    }

    /**
     * Get the current status of the analysis.
     * Returns: "pending", "in-progress", "completed", or "failed"
     */
    public String getStatus() {
        AnalysisResult result = getResult();
        if (result == null) {
            return "pending";
        }
        return result.status;
    }

    /**
     * Check if the page should auto-refresh (when analysis is still running).
     */
    public boolean isAutoRefresh() {
        String status = getStatus();
        return "pending".equals(status) || "in-progress".equals(status);
    }

    /**
     * Read the analysis result from the JSON file produced by the Go binary.
     */
    public AnalysisResult getResult() {
        Path resultFile = Paths.get(RESULTS_DIR, analysisId + ".json");
        if (!Files.exists(resultFile)) {
            return null;
        }

        try {
            String json = Files.readString(resultFile);
            return new Gson().fromJson(json, AnalysisResult.class);
        } catch (IOException e) {
            LOGGER.log(Level.WARNING, "Failed to read analysis result: " + analysisId, e);
            return null;
        }
    }

    /** Data class representing the analysis result read from JSON. */
    public static class AnalysisResult {
        public String status;
        public String category;
        public String rootCauseSummary;
        public String rootCauseDetails;
        public String responsibleTeam;
        public String teamEmail;
        public String htmlReport;
        public String jiraTicketKey;
        public String jiraTicketUrl;
        public String[] evidence;
        public String[] nextSteps;
        public String errorMessage;
        public long analysisTimeMs;
    }
}
