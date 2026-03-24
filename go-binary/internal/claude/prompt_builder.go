package claude

import (
	"fmt"
	"strings"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

const maxConsoleLogLines = 200
const maxErrorMessages = 20

// BuildUserPrompt constructs a structured prompt containing all available build
// failure data for Claude to analyze.
func BuildUserPrompt(buildCtx *models.BuildContext, mcpResults *models.McpResults, corr *models.Correlation) string {
	var b strings.Builder

	writeBuildInfo(&b, buildCtx)
	writeFailedStage(&b, buildCtx)
	writeErrorMessages(&b, buildCtx)
	writeConsoleLog(&b, buildCtx)
	writeJenkinsData(&b, mcpResults)
	writeBitBucketData(&b, mcpResults)
	writeGithubData(&b, mcpResults)
	writeKubernetesData(&b, mcpResults)
	writeDockerData(&b, mcpResults)
	writeJFrogData(&b, mcpResults)
	writeNexusData(&b, mcpResults)
	writeCorrelation(&b, corr)

	return b.String()
}

func writeBuildInfo(b *strings.Builder, ctx *models.BuildContext) {
	b.WriteString("=== BUILD INFORMATION ===\n")
	if ctx == nil {
		b.WriteString("Data not available\n\n")
		return
	}
	fmt.Fprintf(b, "Job Name: %s\n", ctx.JobName)
	fmt.Fprintf(b, "Build Number: %d\n", ctx.BuildNumber)
	fmt.Fprintf(b, "Build URL: %s\n", ctx.BuildUrl)
	fmt.Fprintf(b, "Branch: %s\n", ctx.Branch)
	fmt.Fprintf(b, "Commit: %s\n", ctx.CommitHash)
	b.WriteString("\n")
}

func writeFailedStage(b *strings.Builder, ctx *models.BuildContext) {
	b.WriteString("=== FAILED STAGE ===\n")
	if ctx == nil {
		b.WriteString("Data not available\n\n")
		return
	}
	fmt.Fprintf(b, "Stage: %s\n", ctx.FailedStage)
	fmt.Fprintf(b, "Suspected Repository: %s\n", ctx.SuspectedRepository)
	b.WriteString("\n")
}

func writeErrorMessages(b *strings.Builder, ctx *models.BuildContext) {
	b.WriteString("=== ERROR MESSAGES ===\n")
	if ctx == nil || len(ctx.ErrorMessages) == 0 {
		b.WriteString("No error messages captured\n\n")
		return
	}
	limit := len(ctx.ErrorMessages)
	if limit > maxErrorMessages {
		limit = maxErrorMessages
	}
	for i := 0; i < limit; i++ {
		fmt.Fprintf(b, "%d. %s\n", i+1, ctx.ErrorMessages[i])
	}
	if len(ctx.ErrorMessages) > maxErrorMessages {
		fmt.Fprintf(b, "... and %d more error messages\n", len(ctx.ErrorMessages)-maxErrorMessages)
	}
	b.WriteString("\n")
}

func writeConsoleLog(b *strings.Builder, ctx *models.BuildContext) {
	b.WriteString("=== CONSOLE LOG (last 200 lines) ===\n")
	if ctx == nil || ctx.ConsoleLog == "" {
		b.WriteString("Console log not available\n\n")
		return
	}
	lines := strings.Split(ctx.ConsoleLog, "\n")
	start := 0
	if len(lines) > maxConsoleLogLines {
		start = len(lines) - maxConsoleLogLines
		fmt.Fprintf(b, "... [truncated %d earlier lines] ...\n", start)
	}
	for i := start; i < len(lines); i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func writeJenkinsData(b *strings.Builder, results *models.McpResults) {
	b.WriteString("=== JENKINS AGENT DATA ===\n")
	if results == nil || results.JenkinsResult == nil {
		b.WriteString("Data not available\n\n")
		return
	}
	jr := results.JenkinsResult

	fmt.Fprintf(b, "Agent Online: %v\n", jr.AgentOnline)
	fmt.Fprintf(b, "Agent Disk Free: %s\n", jr.AgentDiskFree)

	if len(jr.Stages) > 0 {
		b.WriteString("Pipeline Stages:\n")
		for _, s := range jr.Stages {
			fmt.Fprintf(b, "  - %s: %s (duration: %dms)\n", s.Name, s.Status, s.DurationMs)
		}
	}

	if len(jr.ErrorArtifacts) > 0 {
		b.WriteString("Error Artifacts:\n")
		for _, a := range jr.ErrorArtifacts {
			fmt.Fprintf(b, "  - %s\n", a)
		}
	}
	b.WriteString("\n")
}

func writeBitBucketData(b *strings.Builder, results *models.McpResults) {
	b.WriteString("=== BITBUCKET DATA ===\n")
	if results == nil || results.BitBucketResult == nil {
		b.WriteString("Data not available\n\n")
		return
	}
	bb := results.BitBucketResult

	if len(bb.RecentCommits) > 0 {
		b.WriteString("Recent Commits:\n")
		for _, c := range bb.RecentCommits {
			fmt.Fprintf(b, "  - [%s] %s by %s (%s)\n", shortHash(c.Hash), c.Message, c.Author, c.Date)
		}
	} else {
		b.WriteString("No recent commits found\n")
	}

	if len(bb.ChangedFiles) > 0 {
		b.WriteString("Changed Files:\n")
		for _, f := range bb.ChangedFiles {
			fmt.Fprintf(b, "  - %s\n", f)
		}
	}

	if bb.CodeOwners != "" {
		fmt.Fprintf(b, "CODEOWNERS:\n%s\n", bb.CodeOwners)
	}
	b.WriteString("\n")
}

func writeGithubData(b *strings.Builder, results *models.McpResults) {
	b.WriteString("=== GITHUB DATA ===\n")
	if results == nil || results.GithubResult == nil {
		b.WriteString("Data not available\n\n")
		return
	}
	gh := results.GithubResult

	if gh.PrTitle != "" {
		fmt.Fprintf(b, "PR Title: %s\n", gh.PrTitle)
	}
	if gh.PrBody != "" {
		fmt.Fprintf(b, "PR Body: %s\n", gh.PrBody)
	}

	if len(gh.RecentCommits) > 0 {
		b.WriteString("Recent Commits:\n")
		for _, c := range gh.RecentCommits {
			fmt.Fprintf(b, "  - [%s] %s by %s (%s)\n", shortHash(c.Hash), c.Message, c.Author, c.Date)
		}
	}

	if len(gh.ChangedFiles) > 0 {
		b.WriteString("Changed Files:\n")
		for _, f := range gh.ChangedFiles {
			fmt.Fprintf(b, "  - %s\n", f)
		}
	}

	if gh.CodeOwners != "" {
		fmt.Fprintf(b, "CODEOWNERS:\n%s\n", gh.CodeOwners)
	}
	b.WriteString("\n")
}

func writeKubernetesData(b *strings.Builder, results *models.McpResults) {
	b.WriteString("=== KUBERNETES DATA ===\n")
	if results == nil || results.KubernetesResult == nil {
		b.WriteString("Data not available\n\n")
		return
	}
	k8s := results.KubernetesResult

	if len(k8s.PodStatuses) > 0 {
		b.WriteString("Pod Statuses:\n")
		for _, p := range k8s.PodStatuses {
			fmt.Fprintf(b, "  - %s: phase=%s reason=%s restarts=%d\n", p.Name, p.Phase, p.Reason, p.RestartCount)
		}
	}

	if len(k8s.OOMKills) > 0 {
		b.WriteString("OOM Kills:\n")
		for _, oom := range k8s.OOMKills {
			fmt.Fprintf(b, "  - %s\n", oom)
		}
	}

	fmt.Fprintf(b, "Node Pressure: %v\n", k8s.NodePressure)

	if len(k8s.Events) > 0 {
		b.WriteString("Events:\n")
		for _, e := range k8s.Events {
			fmt.Fprintf(b, "  - %s\n", e)
		}
	}
	b.WriteString("\n")
}

func writeJFrogData(b *strings.Builder, results *models.McpResults) {
	b.WriteString("=== JFROG DATA ===\n")
	if results == nil || results.JFrogResult == nil {
		b.WriteString("Data not available\n\n")
		return
	}
	jf := results.JFrogResult

	fmt.Fprintf(b, "Artifacts Available: %v\n", jf.ArtifactsAvailable)
	fmt.Fprintf(b, "Repository Status: %s\n", jf.RepositoryStatus)

	if len(jf.MissingArtifacts) > 0 {
		b.WriteString("Missing Artifacts:\n")
		for _, a := range jf.MissingArtifacts {
			fmt.Fprintf(b, "  - %s\n", a)
		}
	}
	b.WriteString("\n")
}

func writeDockerData(b *strings.Builder, results *models.McpResults) {
	b.WriteString("=== DOCKER DATA ===\n")
	if results == nil || results.DockerResult == nil {
		b.WriteString("Data not available\n\n")
		return
	}
	d := results.DockerResult

	if len(d.ContainerStatuses) > 0 {
		b.WriteString("Container Statuses:\n")
		for _, c := range d.ContainerStatuses {
			fmt.Fprintf(b, "  - %s (image=%s): state=%s status=%s exitCode=%d\n", c.Name, c.Image, c.State, c.Status, c.ExitCode)
		}
	}

	if len(d.FailedContainers) > 0 {
		b.WriteString("Failed Containers:\n")
		for _, c := range d.FailedContainers {
			fmt.Fprintf(b, "  - %s\n", c)
		}
	}

	if len(d.OOMKilled) > 0 {
		b.WriteString("OOM Killed Containers:\n")
		for _, c := range d.OOMKilled {
			fmt.Fprintf(b, "  - %s\n", c)
		}
	}

	if len(d.ImageIssues) > 0 {
		b.WriteString("Image Issues:\n")
		for _, i := range d.ImageIssues {
			fmt.Fprintf(b, "  - %s\n", i)
		}
	}

	if d.DiskUsage != "" {
		fmt.Fprintf(b, "Disk Usage: %s\n", d.DiskUsage)
	}
	b.WriteString("\n")
}

func writeNexusData(b *strings.Builder, results *models.McpResults) {
	b.WriteString("=== NEXUS DATA ===\n")
	if results == nil || results.NexusResult == nil {
		b.WriteString("Data not available\n\n")
		return
	}
	nx := results.NexusResult

	fmt.Fprintf(b, "Artifacts Available: %v\n", nx.ArtifactsAvailable)
	fmt.Fprintf(b, "Repository Status: %s\n", nx.RepositoryStatus)

	if len(nx.MissingArtifacts) > 0 {
		b.WriteString("Missing Artifacts:\n")
		for _, a := range nx.MissingArtifacts {
			fmt.Fprintf(b, "  - %s\n", a)
		}
	}
	b.WriteString("\n")
}

func writeCorrelation(b *strings.Builder, corr *models.Correlation) {
	b.WriteString("=== CROSS-CORRELATION ANALYSIS ===\n")
	if corr == nil {
		b.WriteString("Data not available\n\n")
		return
	}
	fmt.Fprintf(b, "Root Cause Type: %s\n", corr.RootCauseType)
	fmt.Fprintf(b, "Is Infrastructure: %v\n", corr.IsInfrastructure)
	fmt.Fprintf(b, "Responsible Repository: %s\n", corr.ResponsibleRepository)
	fmt.Fprintf(b, "Responsible Team: %s\n", corr.ResponsibleTeam)

	if len(corr.Evidence) > 0 {
		b.WriteString("Evidence:\n")
		for _, e := range corr.Evidence {
			fmt.Fprintf(b, "  - %s\n", e)
		}
	}
	b.WriteString("\n")
}

// shortHash returns the first 8 characters of a commit hash.
func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}
