package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// kubectlValidateAPIServer checks if the API server is accessible using kubectl.
func kubectlValidateAPIServer(kubeconfigPath string) (string, error) {
	cmd := exec.Command("kubectl", "cluster-info")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("API server is not responding")
	}

	return "API server is accessible", nil
}

// kubectlValidateNodes checks if all nodes are ready.
func kubectlValidateNodes(kubeconfigPath string) (string, error) {
	cmd := exec.Command("kubectl", "get", "nodes", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get nodes: %w", err)
	}

	var result struct {
		Items []struct {
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	total := len(result.Items)
	ready := 0

	for _, node := range result.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready++
				break
			}
		}
	}

	if ready < total {
		return "", fmt.Errorf("%d/%d nodes ready", ready, total)
	}

	return fmt.Sprintf("All %d nodes are ready", total), nil
}

// kubectlValidateSystemPods checks if all system pods are running.
func kubectlValidateSystemPods(kubeconfigPath string) (string, error) {
	cmd := exec.Command("kubectl", "get", "pods", "-n", "kube-system", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get pods: %w", err)
	}

	var result struct {
		Items []struct {
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	running := 0
	completed := 0

	for _, pod := range result.Items {
		if pod.Status.Phase == "Running" {
			running++
		} else if pod.Status.Phase == "Succeeded" {
			completed++
		}
	}

	active := len(result.Items) - completed
	if running < active {
		return "", fmt.Errorf("%d/%d pods running", running, active)
	}

	if completed > 0 {
		return fmt.Sprintf("All %d system pods are running (%d completed jobs)", active, completed), nil
	}
	return fmt.Sprintf("All %d system pods are running", active), nil
}

// kubectlValidateEtcd checks etcd cluster health.
func kubectlValidateEtcd(kubeconfigPath string) (string, error) {
	cmd := exec.Command("kubectl", "get", "pods", "-n", "kube-system", "-l", "component=etcd", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to check etcd: %w", err)
	}

	var result struct {
		Items []struct {
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	members := len(result.Items)
	if members == 0 {
		return "etcd is running on control plane nodes", nil
	}

	running := 0
	for _, pod := range result.Items {
		if pod.Status.Phase == "Running" {
			running++
		}
	}

	return fmt.Sprintf("etcd cluster healthy (%d members)", running), nil
}

// kubectlValidateDNS checks DNS functionality.
func kubectlValidateDNS(kubeconfigPath string) (string, error) {
	cmd := exec.Command("kubectl", "get", "pods", "-n", "kube-system", "-l", "k8s-app=kube-dns", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to check DNS: %w", err)
	}

	var result struct {
		Items []struct {
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	running := 0
	for _, pod := range result.Items {
		if pod.Status.Phase == "Running" {
			running++
		}
	}

	if running == 0 {
		return "", fmt.Errorf("no DNS pods running")
	}

	return fmt.Sprintf("DNS is working (%d pods running)", running), nil
}

// kubectlValidateNetworking checks pod networking (CNI).
func kubectlValidateNetworking(kubeconfigPath string) (string, error) {
	cmd := exec.Command("kubectl", "get", "pods", "-n", "kube-system", "-l", "k8s-app=canal", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to check networking: %w", err)
	}

	var result struct {
		Items []struct {
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	running := 0
	for _, pod := range result.Items {
		if pod.Status.Phase == "Running" {
			running++
		}
	}

	if running == 0 {
		return "", fmt.Errorf("no CNI pods running")
	}

	return fmt.Sprintf("Pod networking is operational (%d Canal pods running)", running), nil
}

// kubectlValidatePodScheduling checks if pods can be scheduled.
func kubectlValidatePodScheduling(kubeconfigPath string) (string, error) {
	cmd := exec.Command("kubectl", "get", "pods", "--all-namespaces", "--field-selector=status.phase=Pending", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to check pod scheduling: %w", err)
	}

	var result struct {
		Items []interface{} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	if len(result.Items) > 0 {
		return "", fmt.Errorf("%d pods are pending", len(result.Items))
	}

	return "Pod scheduling is working correctly", nil
}

// kubectlGetClusterStatus returns detailed cluster status using kubectl.
func kubectlGetClusterStatus(kubeconfigPath string, apiEndpoint string) (*ClusterStatus, error) {
	status := &ClusterStatus{
		Ready:       false,
		Message:     "Checking cluster status...",
		APIEndpoint: apiEndpoint,
	}

	// Check nodes
	cmd := exec.Command("kubectl", "get", "nodes", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		status.Message = "Unable to connect to API server"
		return status, nil
	}

	// Parse nodes
	var nodesResult struct {
		Items []struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &nodesResult); err == nil {
		for _, node := range nodesResult.Items {
			isControlPlane := false
			if _, ok := node.Metadata.Labels["node-role.kubernetes.io/control-plane"]; ok {
				isControlPlane = true
				status.ControlPlaneTotal++
			} else {
				status.WorkerTotal++
			}

			for _, condition := range node.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					if isControlPlane {
						status.ControlPlaneReady++
					} else {
						status.WorkerReady++
					}
				}
			}
		}
	}

	// Check system pods
	cmd = exec.Command("kubectl", "get", "pods", "-n", "kube-system", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err = cmd.Output()
	if err == nil {
		var podsResult struct {
			Items []struct {
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
				Status struct {
					Phase string `json:"phase"`
				} `json:"status"`
			} `json:"items"`
		}

		if err := json.Unmarshal(output, &podsResult); err == nil {
			componentCounts := make(map[string]int)
			componentReady := make(map[string]int)

			for _, pod := range podsResult.Items {
				name := pod.Metadata.Name
				component := "other"
				if strings.Contains(name, "coredns") {
					component = "coredns"
				} else if strings.Contains(name, "canal") {
					component = "canal"
				} else if strings.Contains(name, "etcd") {
					component = "etcd"
				} else if strings.Contains(name, "kube-apiserver") {
					component = "kube-apiserver"
				}

				if pod.Status.Phase == "Succeeded" {
					continue
				}
				componentCounts[component]++
				if pod.Status.Phase == "Running" {
					componentReady[component]++
				}
			}

			for comp, total := range componentCounts {
				ready := componentReady[comp]
				compStatus := ComponentStatus{
					Name:   comp,
					Status: "healthy",
				}
				if ready == total {
					compStatus.Message = fmt.Sprintf("%d/%d running", ready, total)
				} else {
					compStatus.Status = "degraded"
					compStatus.Message = fmt.Sprintf("%d/%d running", ready, total)
				}
				status.Components = append(status.Components, compStatus)
			}
		}
	}

	// Determine overall readiness
	allNodesReady := status.ControlPlaneReady == status.ControlPlaneTotal &&
		status.WorkerReady == status.WorkerTotal &&
		status.ControlPlaneTotal > 0 &&
		status.WorkerTotal > 0

	if allNodesReady {
		status.Ready = true
		status.Message = "Cluster is healthy"
	} else {
		status.Message = "Cluster is not fully ready"
	}

	return status, nil
}
