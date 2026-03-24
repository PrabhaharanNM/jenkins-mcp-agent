package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// BitBucketAgent queries the Bitbucket Server REST API to collect commit
// history, CODEOWNERS, and changed-file information.
type BitBucketAgent struct {
	req *models.AnalysisRequest
}

// NewBitBucketAgent creates a BitBucketAgent bound to the given request.
func NewBitBucketAgent(req *models.AnalysisRequest) *BitBucketAgent {
	return &BitBucketAgent{req: req}
}

// Analyze fetches CODEOWNERS, recent commits, and changed files from Bitbucket
// for the suspected repository (with higher commit depth) and other repos.
func (a *BitBucketAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.BitBucketAgentResult, error) {
	result := &models.BitBucketAgentResult{}
	cfg := a.req.BitBucket
	baseURL := strings.TrimRight(cfg.Url, "/")
	// Bitbucket Cloud repo access tokens use Bearer auth; Server uses Basic auth.
	var auth string
	if strings.Contains(baseURL, "bitbucket.org") {
		auth = "Bearer " + cfg.Password
	} else {
		auth = basicAuthValue(cfg.Username, cfg.Password)
	}

	// Determine which repositories to query.
	repos := a.req.Repositories
	if len(repos) == 0 && buildCtx.SuspectedRepository != "" {
		repos = []models.RepositoryConfig{{
			Name:      buildCtx.SuspectedRepository,
			Workspace: guessWorkspace(buildCtx.Repository),
			Branch:    buildCtx.Branch,
		}}
	}

	for _, repo := range repos {
		workspace := repo.Workspace
		repoSlug := repo.Name
		branch := repo.Branch
		if branch == "" {
			branch = buildCtx.Branch
		}

		// Fetch CODEOWNERS (only for the first / suspected repo).
		if result.CodeOwners == "" {
			owners, err := a.fetchCodeOwners(ctx, baseURL, workspace, repoSlug, auth)
			if err != nil {
				log.Printf("[BitBucketAgent] CODEOWNERS fetch failed for %s/%s: %v", workspace, repoSlug, err)
			} else {
				result.CodeOwners = owners
			}
		}

		// Determine commit limit: more for the suspected repo.
		limit := 10
		if strings.EqualFold(repoSlug, buildCtx.SuspectedRepository) {
			limit = 50
		}

		// Fetch recent commits.
		commits, err := a.fetchCommits(ctx, baseURL, workspace, repoSlug, branch, limit, auth)
		if err != nil {
			log.Printf("[BitBucketAgent] commits fetch failed for %s/%s: %v", workspace, repoSlug, err)
			continue
		}
		result.RecentCommits = append(result.RecentCommits, commits...)

		// Fetch changed files for the most recent commit.
		if len(commits) > 0 {
			files, err := a.fetchChangedFiles(ctx, baseURL, workspace, repoSlug, commits[0].Hash, auth)
			if err != nil {
				log.Printf("[BitBucketAgent] changed files fetch failed for %s/%s commit %s: %v",
					workspace, repoSlug, commits[0].Hash, err)
			} else {
				result.ChangedFiles = append(result.ChangedFiles, files...)
			}
		}
	}

	return result, nil
}

// fetchCodeOwners retrieves the CODEOWNERS file from the repository root.
// Supports both Bitbucket Cloud (api.bitbucket.org/2.0) and Server (/rest/api/1.0).
func (a *BitBucketAgent) fetchCodeOwners(ctx context.Context, baseURL, workspace, repo, auth string) (string, error) {
	var url string
	if strings.Contains(baseURL, "bitbucket.org") {
		// Bitbucket Cloud: GET /2.0/repositories/{workspace}/{repo}/src/HEAD/CODEOWNERS
		url = fmt.Sprintf("%s/repositories/%s/%s/src/HEAD/CODEOWNERS", baseURL, workspace, repo)
	} else {
		// Bitbucket Server
		url = fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/raw/CODEOWNERS", baseURL, workspace, repo)
	}
	body, err := doRequest(ctx, url, "Authorization", auth)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// fetchCommits retrieves recent commits on the given branch.
// Supports both Bitbucket Cloud and Server APIs.
func (a *BitBucketAgent) fetchCommits(ctx context.Context, baseURL, workspace, repo, branch string, limit int, auth string) ([]models.CommitInfo, error) {
	var url string
	if strings.Contains(baseURL, "bitbucket.org") {
		// Bitbucket Cloud: GET /2.0/repositories/{workspace}/{repo}/commits/{branch}
		url = fmt.Sprintf("%s/repositories/%s/%s/commits/%s?pagelen=%d", baseURL, workspace, repo, branch, limit)
	} else {
		// Bitbucket Server
		url = fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/commits?until=%s&limit=%d", baseURL, workspace, repo, branch, limit)
	}
	body, err := doRequest(ctx, url, "Authorization", auth)
	if err != nil {
		return nil, err
	}

	var resp bbCommitResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing commits response: %w", err)
	}

	commits := make([]models.CommitInfo, 0, len(resp.Values))
	for _, c := range resp.Values {
		author := c.Author.Name
		if author == "" {
			author = c.Author.EmailAddress
		}
		// Bitbucket Cloud uses "hash" field, Server uses "id"
		hash := c.ID
		if hash == "" {
			hash = c.Hash
		}
		commits = append(commits, models.CommitInfo{
			Hash:    hash,
			Author:  author,
			Message: c.Message,
			Date:    fmt.Sprintf("%d", c.AuthorTimestamp),
		})
	}
	return commits, nil
}

// fetchChangedFiles retrieves the list of files changed in a specific commit.
// Supports both Bitbucket Cloud and Server APIs.
func (a *BitBucketAgent) fetchChangedFiles(ctx context.Context, baseURL, workspace, repo, hash, auth string) ([]string, error) {
	var url string
	if strings.Contains(baseURL, "bitbucket.org") {
		// Bitbucket Cloud: GET /2.0/repositories/{workspace}/{repo}/diffstat/{hash}
		url = fmt.Sprintf("%s/repositories/%s/%s/diffstat/%s", baseURL, workspace, repo, hash)
	} else {
		// Bitbucket Server
		url = fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/commits/%s/changes", baseURL, workspace, repo, hash)
	}
	body, err := doRequest(ctx, url, "Authorization", auth)
	if err != nil {
		return nil, err
	}

	if strings.Contains(baseURL, "bitbucket.org") {
		// Bitbucket Cloud diffstat response
		var resp bbCloudDiffstatResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parsing diffstat response: %w", err)
		}
		files := make([]string, 0, len(resp.Values))
		for _, v := range resp.Values {
			if v.New.Path != "" {
				files = append(files, v.New.Path)
			} else if v.Old.Path != "" {
				files = append(files, v.Old.Path)
			}
		}
		return files, nil
	}

	// Bitbucket Server changes response
	var resp bbChangesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing changes response: %w", err)
	}

	files := make([]string, 0, len(resp.Values))
	for _, v := range resp.Values {
		if v.Path.ToString != "" {
			files = append(files, v.Path.ToString)
		}
	}
	return files, nil
}

// guessWorkspace attempts to extract a Bitbucket project/workspace key from a
// full repository URL.
func guessWorkspace(repoURL string) string {
	// Typical Bitbucket Server URL: https://bitbucket.example.com/scm/PROJECT/repo.git
	parts := strings.Split(strings.TrimRight(repoURL, "/"), "/")
	for i, p := range parts {
		if strings.EqualFold(p, "scm") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return ""
}

// --- internal JSON shapes ------------------------------------------------

type bbCommitResponse struct {
	Values []bbCommit `json:"values"`
}

type bbCommit struct {
	ID              string   `json:"id"`              // Bitbucket Server
	Hash            string   `json:"hash"`            // Bitbucket Cloud
	Message         string   `json:"message"`
	Author          bbAuthor `json:"author"`
	AuthorTimestamp int64    `json:"authorTimestamp"`
}

type bbAuthor struct {
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
}

type bbChangesResponse struct {
	Values []bbChange `json:"values"`
}

type bbChange struct {
	Path bbPath `json:"path"`
}

type bbPath struct {
	ToString string `json:"toString"`
}

// Bitbucket Cloud diffstat response shapes
type bbCloudDiffstatResponse struct {
	Values []bbCloudDiffstatEntry `json:"values"`
}

type bbCloudDiffstatEntry struct {
	New bbCloudDiffFile `json:"new"`
	Old bbCloudDiffFile `json:"old"`
}

type bbCloudDiffFile struct {
	Path string `json:"path"`
}
