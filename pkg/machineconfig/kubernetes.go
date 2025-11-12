package machineconfig

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
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

type nodeRoleCounts struct {
	controlPlane       int
	workerOnly         int
	schedulableMasters int
}

// IsClusterSingleNode detects if the cluster is single-node or compact (schedulable masters)
func IsClusterSingleNode(kubeconfigPath string) (useMasterRole bool, info string, err error) {
	clientset, err := getKubernetesClient(kubeconfigPath)
	if err != nil {
		return false, "", err
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, "", fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return false, "", fmt.Errorf("no nodes found in cluster")
	}

	counts := countNodeRoles(nodes.Items)
	info = buildClusterInfo(len(nodes.Items), counts)
	useMasterRole = shouldUseMasterRole(len(nodes.Items), counts)

	return useMasterRole, info, nil
}

func getKubernetesClient(kubeconfigPath string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return clientset, nil
}

func countNodeRoles(nodes []corev1.Node) nodeRoleCounts {
	counts := nodeRoleCounts{}

	for i := range nodes {
		node := &nodes[i]
		isMaster, isWorker := getNodeRoles(node)

		if isMaster {
			counts.controlPlane++
		}

		if isMaster && isWorker {
			counts.schedulableMasters++
		} else if isWorker {
			counts.workerOnly++
		}
	}

	return counts
}

func getNodeRoles(node *corev1.Node) (isMaster, isWorker bool) {
	isMaster = hasLabel(node, "node-role.kubernetes.io/master") ||
		hasLabel(node, "node-role.kubernetes.io/control-plane")
	isWorker = hasLabel(node, "node-role.kubernetes.io/worker")
	return isMaster, isWorker
}

func hasLabel(node *corev1.Node, label string) bool {
	_, ok := node.Labels[label]
	return ok
}

func buildClusterInfo(totalNodes int, counts nodeRoleCounts) string {
	var info strings.Builder
	totalWorkers := counts.workerOnly + counts.schedulableMasters

	info.WriteString(fmt.Sprintf("  Nodes: %d\n", totalNodes))
	info.WriteString(fmt.Sprintf("  Control Plane Nodes: %d\n", counts.controlPlane))
	info.WriteString(fmt.Sprintf("  Worker Nodes (total): %d\n", totalWorkers))

	if counts.schedulableMasters > 0 {
		info.WriteString(fmt.Sprintf("  Schedulable Masters (also workers): %d\n", counts.schedulableMasters))
	}
	if counts.workerOnly > 0 {
		info.WriteString(fmt.Sprintf("  Dedicated Workers: %d\n", counts.workerOnly))
	}

	return info.String()
}

func shouldUseMasterRole(totalNodes int, counts nodeRoleCounts) bool {
	// Use master role if:
	// 1. Single node (1 total node)
	// 2. Compact cluster (only schedulable masters, no dedicated workers)
	return totalNodes == 1 || (counts.schedulableMasters > 0 && counts.workerOnly == 0)
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
	unstructuredMC := toUnstructured(mc)

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

func toUnstructured(mc *MachineConfig) *unstructured.Unstructured {
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
	return obj
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
