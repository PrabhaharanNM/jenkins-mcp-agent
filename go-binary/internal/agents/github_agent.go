package agents

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// GithubAgent queries the GitHub REST API to collect recent commits, changed
// files, and CODEOWNERS information for Jenkins users with GitHub SCM.
type GithubAgent struct {
	req *models.AnalysisRequest
}

// NewGithubAgent creates a GithubAgent bound to the given request.
func NewGithubAgent(req *models.AnalysisRequest) *GithubAgent {
	return &GithubAgent{req: req}
}

// Analyze fetches commit history, changed files, and CODEOWNERS from GitHub.
// If GitHub configuration is not provided, it returns an empty result.
func (a *GithubAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.GithubAgentResult, error) {
	cfg := a.req.GitHub
	if cfg.Token == "" {
		log.Printf("[GithubAgent] GitHub config not provided, skipping")
		return &models.GithubAgentResult{}, nil
	}

	result := &models.GithubAgentResult{}

	apiURL := strings.TrimRight(cfg.ApiUrl, "/")
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}
	auth := "Bearer " + cfg.Token
	owner := cfg.Owner
	repo := cfg.Repo

	// --- Fetch recent commits ---
	commits, err := a.fetchRecentCommits(ctx, apiURL, owner, repo, auth)
	if err != nil {
		log.Printf("[GithubAgent] commits fetch failed for %s/%s: %v", owner, repo, err)
	} else {
		result.RecentCommits = commits
	}

	// --- Fetch changed files from the latest commit ---
	if len(commits) > 0 {
		files, err := a.fetchChangedFiles(ctx, apiURL, owner, repo, commits[0].Hash, auth)
		if err != nil {
			log.Printf("[GithubAgent] changed files fetch failed for %s/%s commit %s: %v",
				owner, repo, commits[0].Hash, err)
		} else {
			result.ChangedFiles = files
		}
	}

	// --- Fetch CODEOWNERS ---
	codeowners, err := a.fetchCodeOwners(ctx, apiURL, owner, repo, auth)
	if err != nil {
		log.Printf("[GithubAgent] CODEOWNERS fetch failed for %s/%s: %v", owner, repo, err)
	} else {
		result.CodeOwners = codeowners
	}

	return result, nil
}

// fetchRecentCommits retrieves the 10 most recent commits from the repository.
func (a *GithubAgent) fetchRecentCommits(ctx context.Context, apiURL, owner, repo, auth string) ([]models.CommitInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits?per_page=10", apiURL, owner, repo)
	body, err := doRequest(ctx, url, "Authorization", auth)
	if err != nil {
		return nil, err
	}

	var ghCommits []ghCommitResponse
	if err := json.Unmarshal(body, &ghCommits); err != nil {
		return nil, fmt.Errorf("parsing commits response: %w", err)
	}

	commits := make([]models.CommitInfo, 0, len(ghCommits))
	for _, c := range ghCommits {
		author := c.Commit.Author.Name
		if author == "" && c.Author.Login != "" {
			author = c.Author.Login
		}
		commits = append(commits, models.CommitInfo{
			Hash:    c.SHA,
			Author:  author,
			Message: c.Commit.Message,
			Date:    c.Commit.Author.Date,
		})
	}
	return commits, nil
}

// fetchChangedFiles retrieves the list of files changed in a specific commit.
func (a *GithubAgent) fetchChangedFiles(ctx context.Context, apiURL, owner, repo, sha, auth string) ([]string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s", apiURL, owner, repo, sha)
	body, err := doRequest(ctx, url, "Authorization", auth)
	if err != nil {
		return nil, err
	}

	var detail ghCommitDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, fmt.Errorf("parsing commit detail: %w", err)
	}

	files := make([]string, 0, len(detail.Files))
	for _, f := range detail.Files {
		files = append(files, f.Filename)
	}
	return files, nil
}

// fetchCodeOwners attempts to retrieve the CODEOWNERS file from the repository.
// It tries the root path first, then falls back to .github/CODEOWNERS.
func (a *GithubAgent) fetchCodeOwners(ctx context.Context, apiURL, owner, repo, auth string) (string, error) {
	// Try root CODEOWNERS first.
	url := fmt.Sprintf("%s/repos/%s/%s/contents/CODEOWNERS", apiURL, owner, repo)
	content, err := a.fetchFileContent(ctx, url, auth)
	if err == nil {
		return content, nil
	}

	// Fall back to .github/CODEOWNERS.
	url = fmt.Sprintf("%s/repos/%s/%s/contents/.github/CODEOWNERS", apiURL, owner, repo)
	content, err = a.fetchFileContent(ctx, url, auth)
	if err != nil {
		return "", err
	}
	return content, nil
}

// fetchFileContent retrieves a file from the GitHub Contents API and decodes
// its base64-encoded content.
func (a *GithubAgent) fetchFileContent(ctx context.Context, url, auth string) (string, error) {
	body, err := doRequest(ctx, url, "Authorization", auth)
	if err != nil {
		return "", err
	}

	var file ghFileContent
	if err := json.Unmarshal(body, &file); err != nil {
		return "", fmt.Errorf("parsing file content: %w", err)
	}

	if file.Encoding == "base64" {
		// GitHub returns base64-encoded content with newlines; decode it.
		cleaned := strings.ReplaceAll(file.Content, "\n", "")
		decoded, err := decodeBase64Content(cleaned)
		if err != nil {
			return "", fmt.Errorf("decoding base64 content: %w", err)
		}
		return string(decoded), nil
	}

	return file.Content, nil
}

// decodeBase64Content decodes a standard base64-encoded string.
func decodeBase64Content(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// --- internal JSON shapes ------------------------------------------------

type ghCommitResponse struct {
	SHA    string       `json:"sha"`
	Commit ghCommitData `json:"commit"`
	Author ghUser       `json:"author"`
}

type ghCommitData struct {
	Message string       `json:"message"`
	Author  ghAuthorData `json:"author"`
}

type ghAuthorData struct {
	Name string `json:"name"`
	Date string `json:"date"`
}

type ghUser struct {
	Login string `json:"login"`
}

type ghCommitDetail struct {
	Files []ghFile `json:"files"`
}

type ghFile struct {
	Filename string `json:"filename"`
}

type ghFileContent struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}
