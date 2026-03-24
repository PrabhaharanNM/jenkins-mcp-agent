package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/PrabhaharanNM/jenkins-mcp-agent/go-binary/internal/models"
)

// KubernetesAgent queries the Kubernetes API to collect pod statuses, OOM
// events, node pressure conditions, and cluster events related to the build.
type KubernetesAgent struct {
	req *models.AnalysisRequest
}

// NewKubernetesAgent creates a KubernetesAgent bound to the given request.
func NewKubernetesAgent(req *models.AnalysisRequest) *KubernetesAgent {
	return &KubernetesAgent{req: req}
}

// Analyze fetches pod, event, and node data from the Kubernetes API. If the
// Kubernetes configuration is not provided, it returns an empty result.
func (a *KubernetesAgent) Analyze(ctx context.Context, buildCtx *models.BuildContext) (*models.KubernetesAgentResult, error) {
	cfg := a.req.Kubernetes
	if cfg.ApiUrl == "" || cfg.Token == "" {
		log.Printf("[KubernetesAgent] Kubernetes config not provided, skipping")
		return &models.KubernetesAgentResult{}, nil
	}

	result := &models.KubernetesAgentResult{}
	baseURL := strings.TrimRight(cfg.ApiUrl, "/")
	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "default"
	}
	auth := "Bearer " + cfg.Token

	// --- Fetch pods matching the job ---
	pods, oomKills, err := a.fetchPods(ctx, baseURL, namespace, buildCtx.JobName, auth)
	if err != nil {
		log.Printf("[KubernetesAgent] pod fetch failed: %v", err)
	} else {
		result.PodStatuses = pods
		result.OOMKills = oomKills
	}

	// --- Fetch events related to the agent ---
	if buildCtx.AgentName != "" {
		events, err := a.fetchEvents(ctx, baseURL, namespace, buildCtx.AgentName, auth)
		if err != nil {
			log.Printf("[KubernetesAgent] events fetch failed: %v", err)
		} else {
			result.Events = events
		}
	}

	// --- Check node conditions ---
	nodeName := a.extractNodeName(result.PodStatuses, ctx, baseURL, namespace, buildCtx.JobName, auth)
	if nodeName != "" {
		pressure, err := a.checkNodePressure(ctx, baseURL, nodeName, auth)
		if err != nil {
			log.Printf("[KubernetesAgent] node pressure check failed: %v", err)
		} else {
			result.NodePressure = pressure
		}
	}

	return result, nil
}

// fetchPods queries pods matching the given job label and extracts statuses
// and any OOMKilled containers.
func (a *KubernetesAgent) fetchPods(ctx context.Context, baseURL, namespace, jobName, auth string) ([]models.PodStatus, []string, error) {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/pods?labelSelector=app=%s",
		baseURL, namespace, jobName)
	body, err := doRequest(ctx, url, "Authorization", auth)
	if err != nil {
		return nil, nil, err
	}

	var podList k8sPodList
	if err := json.Unmarshal(body, &podList); err != nil {
		return nil, nil, fmt.Errorf("parsing pod list: %w", err)
	}

	var statuses []models.PodStatus
	var oomKills []string

	for _, pod := range podList.Items {
		ps := models.PodStatus{
			Name:  pod.Metadata.Name,
			Phase: pod.Status.Phase,
		}

		for _, cs := range pod.Status.ContainerStatuses {
			ps.RestartCount += cs.RestartCount
			if cs.State.Terminated != nil && cs.State.Terminated.Reason == "OOMKilled" {
				ps.Reason = "OOMKilled"
				oomKills = append(oomKills, fmt.Sprintf("%s/%s", pod.Metadata.Name, cs.Name))
			}
			if cs.LastState.Terminated != nil && cs.LastState.Terminated.Reason == "OOMKilled" {
				oomKills = append(oomKills, fmt.Sprintf("%s/%s (previous)", pod.Metadata.Name, cs.Name))
			}
		}

		statuses = append(statuses, ps)
	}

	return statuses, oomKills, nil
}

// fetchEvents retrieves Kubernetes events related to the specified object name.
func (a *KubernetesAgent) fetchEvents(ctx context.Context, baseURL, namespace, objectName, auth string) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/events?fieldSelector=involvedObject.name=%s",
		baseURL, namespace, objectName)
	body, err := doRequest(ctx, url, "Authorization", auth)
	if err != nil {
		return nil, err
	}

	var eventList k8sEventList
	if err := json.Unmarshal(body, &eventList); err != nil {
		return nil, fmt.Errorf("parsing events: %w", err)
	}

	events := make([]string, 0, len(eventList.Items))
	for _, e := range eventList.Items {
		events = append(events, fmt.Sprintf("[%s] %s: %s", e.Type, e.Reason, e.Message))
	}
	return events, nil
}

// checkNodePressure checks whether the node reports MemoryPressure or
// DiskPressure conditions.
func (a *KubernetesAgent) checkNodePressure(ctx context.Context, baseURL, nodeName, auth string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/nodes/%s", baseURL, nodeName)
	body, err := doRequest(ctx, url, "Authorization", auth)
	if err != nil {
		return false, err
	}

	var node k8sNode
	if err := json.Unmarshal(body, &node); err != nil {
		return false, fmt.Errorf("parsing node: %w", err)
	}

	for _, cond := range node.Status.Conditions {
		if (cond.Type == "MemoryPressure" || cond.Type == "DiskPressure") && cond.Status == "True" {
			return true, nil
		}
	}
	return false, nil
}

// extractNodeName tries to find the node name from pod spec in the already-
// fetched pod list. If not available, it re-fetches.
func (a *KubernetesAgent) extractNodeName(pods []models.PodStatus, ctx context.Context, baseURL, namespace, jobName, auth string) string {
	// Re-fetch the raw pod list to get nodeName (not stored in models.PodStatus).
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/pods?labelSelector=app=%s",
		baseURL, namespace, jobName)
	body, err := doRequest(ctx, url, "Authorization", auth)
	if err != nil {
		return ""
	}
	var podList k8sPodList
	if err := json.Unmarshal(body, &podList); err != nil {
		return ""
	}
	for _, pod := range podList.Items {
		if pod.Spec.NodeName != "" {
			return pod.Spec.NodeName
		}
	}
	return ""
}

// --- internal JSON shapes ------------------------------------------------

type k8sPodList struct {
	Items []k8sPod `json:"items"`
}

type k8sPod struct {
	Metadata k8sMetadata `json:"metadata"`
	Spec     k8sPodSpec  `json:"spec"`
	Status   k8sPodStat  `json:"status"`
}

type k8sMetadata struct {
	Name string `json:"name"`
}

type k8sPodSpec struct {
	NodeName string `json:"nodeName"`
}

type k8sPodStat struct {
	Phase             string              `json:"phase"`
	ContainerStatuses []k8sContainerStat  `json:"containerStatuses"`
}

type k8sContainerStat struct {
	Name         string         `json:"name"`
	RestartCount int            `json:"restartCount"`
	State        k8sStateWrap   `json:"state"`
	LastState    k8sStateWrap   `json:"lastState"`
}

type k8sStateWrap struct {
	Terminated *k8sTerminated `json:"terminated,omitempty"`
}

type k8sTerminated struct {
	Reason string `json:"reason"`
}

type k8sEventList struct {
	Items []k8sEvent `json:"items"`
}

type k8sEvent struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type k8sNode struct {
	Status k8sNodeStatus `json:"status"`
}

type k8sNodeStatus struct {
	Conditions []k8sCondition `json:"conditions"`
}

type k8sCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}
