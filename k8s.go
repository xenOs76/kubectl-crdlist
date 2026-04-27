package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

type k8sClient struct {
	dynamic   dynamic.Interface
	discovery discovery.DiscoveryInterface
}

func initK8sClient(flags *genericclioptions.ConfigFlags) (*k8sClient, error) {
	config, err := flags.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get rest config: %w", err)
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	return &k8sClient{
		dynamic:   dyn,
		discovery: disco,
	}, nil
}

// crdInfo holds basic info about a CRD.
type crdInfo struct {
	name       string
	group      string
	version    string
	resource   string
	namespaced bool
}

// listCRDs fetches all available CRDs in the cluster.
func (k *k8sClient) listCRDs() ([]crdInfo, error) {
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	list, err := k.dynamic.Resource(gvr).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	var crds []crdInfo

	for _, item := range list.Items {
		info, ok := k.extractCRDInfo(item.Object)
		if ok {
			crds = append(crds, info)
		}
	}

	return crds, nil
}

func (k *k8sClient) extractCRDInfo(obj map[string]any) (crdInfo, bool) {
	name, ok := getString(obj, "metadata", "name")
	if !ok {
		return crdInfo{}, false
	}

	group, _ := getString(obj, "spec", "group")
	plural, _ := getString(obj, "spec", "names", "plural")

	versions, ok := getSlice(obj, "spec", "versions")
	if !ok {
		return crdInfo{}, false
	}

	scope, _ := getString(obj, "spec", "scope")
	namespaced := scope != "Cluster"

	return crdInfo{
		name:       name,
		group:      group,
		version:    k.findPreferredVersion(versions),
		resource:   plural,
		namespaced: namespaced,
	}, true
}

func (k *k8sClient) findPreferredVersion(versions []any) string {
	for _, v := range versions {
		if verName, ok := k.isServedAndStored(v); ok {
			return verName
		}
	}

	if len(versions) > 0 {
		if verMap, ok := versions[0].(map[string]any); ok {
			if vName, ok := verMap["name"].(string); ok {
				return vName
			}
		}
	}

	return ""
}

func (*k8sClient) isServedAndStored(v any) (string, bool) {
	verMap, ok := v.(map[string]any)
	if !ok {
		return "", false
	}

	vName, vNameOk := verMap["name"].(string)
	vServed, vServedOk := verMap["served"].(bool)
	vStorage, vStorageOk := verMap["storage"].(bool)

	if vNameOk && vServedOk && vStorageOk && vServed && vStorage {
		return vName, true
	}

	return "", false
}

// resourceInfo holds info about a specific resource instance.
type resourceInfo struct {
	name      string
	namespace string
	crd       crdInfo
}

// listResources fetches resources for a specific CRD.
func (k *k8sClient) listResources(crd crdInfo, namespace string) ([]resourceInfo, error) {
	gvr := schema.GroupVersionResource{
		Group:    crd.group,
		Version:  crd.version,
		Resource: crd.resource,
	}

	listClient := k.dynamic.Resource(gvr)

	var list *unstructured.UnstructuredList

	var err error

	if crd.namespaced {
		list, err = listClient.Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	} else {
		list, err = listClient.List(context.Background(), metav1.ListOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list resources for %s (namespaced: %v) in namespace %s: %w",
			crd.name, crd.namespaced, namespace, err)
	}

	var resources []resourceInfo

	for _, item := range list.Items {
		resources = append(resources, resourceInfo{
			name:      item.GetName(),
			namespace: item.GetNamespace(),
			crd:       crd,
		})
	}

	return resources, nil
}

// fetchResourceYAML fetches a resource and returns its YAML representation.
// Strictly READ-ONLY.
func (k *k8sClient) fetchResourceYAML(crd crdInfo, name, namespace string) (string, error) {
	gvr := schema.GroupVersionResource{
		Group:    crd.group,
		Version:  crd.version,
		Resource: crd.resource,
	}

	obj, err := k.dynamic.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get resource %s/%s: %w", namespace, name, err)
	}

	// Remove managed fields for cleaner output
	obj.SetManagedFields(nil)

	yamlData, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", fmt.Errorf("failed to marshal to YAML: %w", err)
	}

	return string(yamlData), nil
}

func getString(obj map[string]any, path ...string) (string, bool) {
	var current any = obj

	for _, p := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return "", false
		}

		current, ok = m[p]
		if !ok {
			return "", false
		}
	}

	s, ok := current.(string)

	return s, ok
}

func getSlice(obj map[string]any, path ...string) ([]any, bool) {
	var current any = obj

	for _, p := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}

		current, ok = m[p]
		if !ok {
			return nil, false
		}
	}

	s, ok := current.([]any)

	return s, ok
}
