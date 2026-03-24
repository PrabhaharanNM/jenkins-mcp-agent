package agents

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// DockerAgent queries the Docker Engine REST API to collect container statuses,
// OOM kills, image issues, and disk usage information.
type DockerAgent struct {
	req *models.AnalysisRequest
}

// NewDockerAgent creates a DockerAgent bound to the given request.
func NewDockerAgent(req *models.AnalysisRequest) *DockerAgent {
	return &DockerAgent{req: req}
}

// Analyze fetches container, image, and disk data from the Docker Engine API.
// If Docker configuration is not provided, it returns an empty result.
func (a *DockerAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.DockerAgentResult, error) {
	cfg := a.req.Docker
	if cfg.Host == "" {
		log.Printf("[DockerAgent] Docker config not provided, skipping")
		return &models.DockerAgentResult{}, nil
	}

	result := &models.DockerAgentResult{}

	client, baseURL, err := a.buildClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("building Docker HTTP client: %w", err)
	}

	// --- List all containers (including stopped) ---
	containers, err := a.listContainers(ctx, client, baseURL)
	if err != nil {
		log.Printf("[DockerAgent] container list failed: %v", err)
	} else {
		for _, c := range containers {
			name := c.ID[:12]
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			cs := models.ContainerStatus{
				Name:   name,
				Image:  c.Image,
				State:  c.State,
				Status: c.Status,
			}
			result.ContainerStatuses = append(result.ContainerStatuses, cs)

			// Track failed/exited containers for deeper inspection.
			if c.State == "exited" || c.State == "dead" {
				result.FailedContainers = append(result.FailedContainers, name)
				a.inspectContainer(ctx, client, baseURL, c.ID, name, result)
			}
		}
	}

	// --- Check system disk usage ---
	diskSummary, err := a.fetchDiskUsage(ctx, client, baseURL)
	if err != nil {
		log.Printf("[DockerAgent] disk usage fetch failed: %v", err)
	} else {
		result.DiskUsage = diskSummary
	}

	return result, nil
}

// buildClient creates an http.Client appropriate for the Docker host type.
// For Unix sockets it configures a custom dialer; for TCP with TLS it loads
// the certificate material from DockerConfig.
func (a *DockerAgent) buildClient(cfg models.DockerConfig) (*http.Client, string, error) {
	host := cfg.Host

	// Unix socket: unix:///var/run/docker.sock
	if strings.HasPrefix(host, "unix://") {
		socketPath := strings.TrimPrefix(host, "unix://")
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.DialTimeout("unix", socketPath, 10*time.Second)
			},
		}
		client := &http.Client{Timeout: httpTimeout, Transport: transport}
		// For Unix sockets, use http://localhost as the base URL; the transport
		// ignores the host and dials the socket directly.
		return client, "http://localhost", nil
	}

	// TCP host, possibly with TLS.
	// Convert tcp:// scheme to http:// for the Go HTTP client.
	if strings.HasPrefix(host, "tcp://") {
		host = "http://" + strings.TrimPrefix(host, "tcp://")
	}
	baseURL := strings.TrimRight(host, "/")

	if cfg.TlsCert != "" && cfg.TlsKey != "" {
		cert, err := tls.X509KeyPair([]byte(cfg.TlsCert), []byte(cfg.TlsKey))
		if err != nil {
			return nil, "", fmt.Errorf("loading TLS client cert/key: %w", err)
		}

		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		if cfg.TlsCaCert != "" {
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM([]byte(cfg.TlsCaCert)) {
				return nil, "", fmt.Errorf("failed to parse CA certificate")
			}
			tlsCfg.RootCAs = caCertPool
		}

		transport := &http.Transport{TLSClientConfig: tlsCfg}
		client := &http.Client{Timeout: httpTimeout, Transport: transport}
		return client, baseURL, nil
	}

	// Plain TCP without TLS.
	return &http.Client{Timeout: httpTimeout}, baseURL, nil
}

// listContainers fetches all containers (including stopped ones) from the
// Docker Engine API.
func (a *DockerAgent) listContainers(ctx context.Context, client *http.Client, baseURL string) ([]dockerContainer, error) {
	url := fmt.Sprintf("%s/v1.43/containers/json?all=true", baseURL)
	body, err := doDockerRequest(ctx, client, url)
	if err != nil {
		return nil, err
	}

	var containers []dockerContainer
	if err := json.Unmarshal(body, &containers); err != nil {
		return nil, fmt.Errorf("parsing container list: %w", err)
	}
	return containers, nil
}

// inspectContainer fetches detailed information about a specific container and
// updates the result with OOM and exit code data.
func (a *DockerAgent) inspectContainer(ctx context.Context, client *http.Client, baseURL, containerID, name string, result *models.DockerAgentResult) {
	url := fmt.Sprintf("%s/v1.43/containers/%s/json", baseURL, containerID)
	body, err := doDockerRequest(ctx, client, url)
	if err != nil {
		log.Printf("[DockerAgent] inspect failed for %s: %v", name, err)
		return
	}

	var info dockerInspect
	if err := json.Unmarshal(body, &info); err != nil {
		log.Printf("[DockerAgent] parsing inspect for %s: %v", name, err)
		return
	}

	// Update the exit code in ContainerStatuses.
	for i := range result.ContainerStatuses {
		if result.ContainerStatuses[i].Name == name {
			result.ContainerStatuses[i].ExitCode = info.State.ExitCode
			break
		}
	}

	if info.State.OOMKilled {
		result.OOMKilled = append(result.OOMKilled, name)
	}

	// Check for image-related issues (e.g., image not found would appear in
	// error field or as a specific exit code pattern).
	if info.State.Error != "" {
		result.ImageIssues = append(result.ImageIssues, fmt.Sprintf("%s: %s", name, info.State.Error))
	}
}

// fetchDiskUsage queries the Docker system disk usage endpoint and returns a
// human-readable summary string.
func (a *DockerAgent) fetchDiskUsage(ctx context.Context, client *http.Client, baseURL string) (string, error) {
	url := fmt.Sprintf("%s/v1.43/system/df", baseURL)
	body, err := doDockerRequest(ctx, client, url)
	if err != nil {
		return "", err
	}

	var df dockerDiskUsage
	if err := json.Unmarshal(body, &df); err != nil {
		return "", fmt.Errorf("parsing disk usage: %w", err)
	}

	var totalSize int64
	var reclaimable int64
	for _, img := range df.Images {
		totalSize += img.Size
		if img.Containers == 0 {
			reclaimable += img.Size
		}
	}
	for _, vol := range df.Volumes {
		totalSize += vol.UsageData.Size
		reclaimable += vol.UsageData.RefCount
	}

	return fmt.Sprintf("Images: %d (%.1f GB), Containers: %d, Volumes: %d, Reclaimable: %.1f GB",
		len(df.Images), float64(totalSize)/(1024*1024*1024),
		len(df.Containers), len(df.Volumes),
		float64(reclaimable)/(1024*1024*1024)), nil
}

// doDockerRequest performs an HTTP GET using the given client (which may be
// configured for Unix sockets or TLS).
func doDockerRequest(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", url, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned status %d: %s", url, resp.StatusCode, string(body))
	}
	return body, nil
}

// --- internal JSON shapes ------------------------------------------------

type dockerContainer struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	Image string   `json:"Image"`
	State string   `json:"State"`
	Status string  `json:"Status"`
}

type dockerInspect struct {
	State dockerState `json:"State"`
}

type dockerState struct {
	ExitCode  int    `json:"ExitCode"`
	OOMKilled bool   `json:"OOMKilled"`
	Error     string `json:"Error"`
}

type dockerDiskUsage struct {
	Images     []dockerImageDU     `json:"Images"`
	Containers []dockerContainerDU `json:"Containers"`
	Volumes    []dockerVolumeDU    `json:"Volumes"`
}

type dockerImageDU struct {
	Size       int64 `json:"Size"`
	Containers int   `json:"Containers"`
}

type dockerContainerDU struct {
	SizeRw int64 `json:"SizeRw"`
}

type dockerVolumeDU struct {
	UsageData dockerUsageData `json:"UsageData"`
}

type dockerUsageData struct {
	Size     int64 `json:"Size"`
	RefCount int64 `json:"RefCount"`
}
