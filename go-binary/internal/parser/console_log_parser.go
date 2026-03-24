package parser

import (
	"regexp"
	"strings"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

var (
	repoClone   = regexp.MustCompile(`Cloning repository\s+(.+)`)
	repoFetch   = regexp.MustCompile(`Fetching upstream changes from\s+(.+)`)
	branchRev   = regexp.MustCompile(`Checking out Revision\s+\w+\s+\((.+?)\)`)
	branchGit   = regexp.MustCompile(`> git checkout\s+(.+)`)
	commitHash  = regexp.MustCompile(`Checking out Revision\s+(\w{40})`)
	agentName   = regexp.MustCompile(`Running on\s+(.+?)\s+in`)
	failedStage = regexp.MustCompile(`Stage '(.+?)' failed`)
	pipelineStg = regexp.MustCompile(`\[Pipeline\]\s+\{\s+\((.+?)\)`)
	failedTest1 = regexp.MustCompile(`FAILED\s+(.+)`)
	failedTest2 = regexp.MustCompile(`Tests run:.*Failures:\s*[1-9]`)

	errorPatterns = []string{"error:", "ERROR:", "FATAL:", "Exception", "failed:"}
)

// Parse extracts build context information from a Jenkins console log using
// regex-based heuristics.
func Parse(consoleLog string) *models.BuildContext {
	ctx := &models.BuildContext{
		ConsoleLog:       consoleLog,
		StageRepoMapping: make(map[string]string),
	}

	lines := strings.Split(consoleLog, "\n")

	// Extract repository
	if m := repoClone.FindStringSubmatch(consoleLog); len(m) > 1 {
		ctx.Repository = strings.TrimSpace(m[1])
	} else if m := repoFetch.FindStringSubmatch(consoleLog); len(m) > 1 {
		ctx.Repository = strings.TrimSpace(m[1])
	}

	// Extract branch
	if m := branchRev.FindStringSubmatch(consoleLog); len(m) > 1 {
		ctx.Branch = strings.TrimSpace(m[1])
	} else if m := branchGit.FindStringSubmatch(consoleLog); len(m) > 1 {
		ctx.Branch = strings.TrimSpace(m[1])
	}

	// Extract commit hash
	if m := commitHash.FindStringSubmatch(consoleLog); len(m) > 1 {
		ctx.CommitHash = strings.TrimSpace(m[1])
	}

	// Extract agent name
	if m := agentName.FindStringSubmatch(consoleLog); len(m) > 1 {
		ctx.AgentName = strings.TrimSpace(m[1])
	}

	// Extract failed stage
	if m := failedStage.FindStringSubmatch(consoleLog); len(m) > 1 {
		ctx.FailedStage = strings.TrimSpace(m[1])
	} else {
		// Fallback: find stage name from [Pipeline] marker preceding error lines
		ctx.FailedStage = findFailedStageFromPipeline(lines)
	}

	// Extract all stages
	allMatches := pipelineStg.FindAllStringSubmatch(consoleLog, -1)
	seen := make(map[string]bool)
	for _, m := range allMatches {
		name := strings.TrimSpace(m[1])
		if !seen[name] {
			ctx.AllStages = append(ctx.AllStages, name)
			seen[name] = true
		}
	}

	// Extract error messages (unique, max 20)
	ctx.ErrorMessages = extractErrors(lines)

	// Extract failed tests
	ctx.FailedTests = extractFailedTests(lines)

	// Derive suspected repository from failed stage name
	ctx.SuspectedRepository = deriveSuspectedRepo(ctx.FailedStage)

	// Build stage-to-repo mapping based on naming convention
	ctx.StageRepoMapping = buildStageRepoMapping(ctx.AllStages)

	return ctx
}

// findFailedStageFromPipeline looks for the last [Pipeline] stage marker that
// appears before any error line in the log.
func findFailedStageFromPipeline(lines []string) string {
	currentStage := ""
	for _, line := range lines {
		if m := pipelineStg.FindStringSubmatch(line); len(m) > 1 {
			currentStage = strings.TrimSpace(m[1])
		}
		for _, pat := range errorPatterns {
			if strings.Contains(line, pat) {
				if currentStage != "" {
					return currentStage
				}
			}
		}
	}
	return ""
}

// extractErrors collects unique error lines from the console log, capped at 20.
func extractErrors(lines []string) []string {
	seen := make(map[string]bool)
	var errors []string
	for _, line := range lines {
		if len(errors) >= 20 {
			break
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		for _, pat := range errorPatterns {
			if strings.Contains(trimmed, pat) {
				if !seen[trimmed] {
					seen[trimmed] = true
					errors = append(errors, trimmed)
				}
				break
			}
		}
	}
	return errors
}

// extractFailedTests finds lines that indicate test failures.
func extractFailedTests(lines []string) []string {
	var tests []string
	seen := make(map[string]bool)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := failedTest1.FindStringSubmatch(trimmed); len(m) > 1 {
			name := strings.TrimSpace(m[1])
			if !seen[name] {
				seen[name] = true
				tests = append(tests, name)
			}
		}
		if failedTest2.MatchString(trimmed) && !seen[trimmed] {
			seen[trimmed] = true
			tests = append(tests, trimmed)
		}
	}
	return tests
}

// deriveSuspectedRepo extracts a repository name from the failed stage name
// using common naming patterns like "Build - XXX" or "XXX Build".
func deriveSuspectedRepo(failedStage string) string {
	if failedStage == "" {
		return ""
	}

	// Pattern: "Build - XXX"
	if strings.Contains(failedStage, " - ") {
		parts := strings.SplitN(failedStage, " - ", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}

	// Pattern: "XXX Build"
	if strings.HasSuffix(failedStage, " Build") {
		return strings.TrimSpace(strings.TrimSuffix(failedStage, " Build"))
	}

	return ""
}

// buildStageRepoMapping creates a mapping from stage names to repository names
// based on the convention that stage names contain the repo name.
func buildStageRepoMapping(stages []string) map[string]string {
	mapping := make(map[string]string)
	for _, stage := range stages {
		repo := deriveSuspectedRepo(stage)
		if repo != "" {
			mapping[stage] = repo
		}
	}
	return mapping
}
