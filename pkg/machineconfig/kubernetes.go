package machineconfig

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var machineConfigGVR = schema.GroupVersionResource{
	Group:    "machineconfiguration.openshift.io",
	Version:  "v1",
	Resource: "machineconfigs",
}

// IsClusterSingleNode detects if the cluster is single-node or compact (schedulable masters)
func IsClusterSingleNode(kubeconfigPath string) (bool, string, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to build config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false, "", fmt.Errorf("failed to create clientset: %w", err)
	}

	// Get all nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, "", fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return false, "", fmt.Errorf("no nodes found in cluster")
	}

	// Build cluster info
	var info strings.Builder
	info.WriteString(fmt.Sprintf("  Nodes: %d\n", len(nodes.Items)))

	controlPlaneCount := 0
	workerOnlyCount := 0
	schedulableMasterCount := 0

	for _, node := range nodes.Items {
		isMaster := false
		isWorker := false

		// Check if it's a control plane/master node
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
			isMaster = true
			controlPlaneCount++
		} else if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
			isMaster = true
			controlPlaneCount++
		}

		// Check if it's a worker node
		if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok {
			isWorker = true
		}

		// Count schedulable masters (both master and worker)
		if isMaster && isWorker {
			schedulableMasterCount++
		} else if isWorker {
			workerOnlyCount++
		}
	}

	totalWorkers := workerOnlyCount + schedulableMasterCount

	info.WriteString(fmt.Sprintf("  Control Plane Nodes: %d\n", controlPlaneCount))
	info.WriteString(fmt.Sprintf("  Worker Nodes (total): %d\n", totalWorkers))
	if schedulableMasterCount > 0 {
		info.WriteString(fmt.Sprintf("  Schedulable Masters (also workers): %d\n", schedulableMasterCount))
	}
	if workerOnlyCount > 0 {
		info.WriteString(fmt.Sprintf("  Dedicated Workers: %d\n", workerOnlyCount))
	}

	// Use master role if:
	// 1. Single node (1 total node)
	// 2. Compact cluster (only schedulable masters, no dedicated workers)
	useMasterRole := len(nodes.Items) == 1 || (schedulableMasterCount > 0 && workerOnlyCount == 0)

	return useMasterRole, info.String(), nil
}

// ApplyMachineConfig applies a MachineConfig to the cluster
func ApplyMachineConfig(ctx context.Context, kubeconfigPath string, mc *MachineConfig) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to build config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Convert MachineConfig to unstructured
	unstructuredMC, err := toUnstructured(mc)
	if err != nil {
		return fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	// Try to get existing MachineConfig
	existing, err := dynamicClient.Resource(machineConfigGVR).Get(ctx, mc.Metadata.Name, metav1.GetOptions{})
	if err == nil {
		// Update existing
		unstructuredMC.SetResourceVersion(existing.GetResourceVersion())
		_, err = dynamicClient.Resource(machineConfigGVR).Update(ctx, unstructuredMC, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update MachineConfig: %w", err)
		}
		fmt.Printf("Updated existing MachineConfig: %s\n", mc.Metadata.Name)
	} else {
		// Create new
		_, err = dynamicClient.Resource(machineConfigGVR).Create(ctx, unstructuredMC, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create MachineConfig: %w", err)
		}
		fmt.Printf("Created new MachineConfig: %s\n", mc.Metadata.Name)
	}

	return nil
}

func toUnstructured(mc *MachineConfig) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": mc.APIVersion,
			"kind":       mc.Kind,
			"metadata": map[string]interface{}{
				"name":   mc.Metadata.Name,
				"labels": mc.Metadata.Labels,
			},
			"spec": map[string]interface{}{
				"config": map[string]interface{}{
					"ignition": map[string]interface{}{
						"version": mc.Spec.Config.Ignition.Version,
					},
					"storage": map[string]interface{}{
						"files": convertFiles(mc.Spec.Config.Storage.Files),
					},
				},
			},
		},
	}
	return obj, nil
}

func convertFiles(files []File) []interface{} {
	result := make([]interface{}, len(files))
	for i, f := range files {
		result[i] = map[string]interface{}{
			"path":      f.Path,
			"mode":      f.Mode,
			"overwrite": f.Overwrite,
			"contents": map[string]interface{}{
				"source": f.Contents.Source,
			},
		}
	}
	return result
}
