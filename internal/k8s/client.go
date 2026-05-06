package k8s

import (
	"context"
	"fmt"

	"github.com/xenos76/kubectl-crdlist/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

type Client struct {
	Dynamic   dynamic.Interface
	Discovery discovery.DiscoveryInterface
}

func NewClient(flags *genericclioptions.ConfigFlags) (*Client, error) {
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

	return &Client{
		Dynamic:   dyn,
		Discovery: disco,
	}, nil
}

// ListCRDs fetches all available CRDs in the cluster.
func (k *Client) ListCRDs() ([]model.CRDInfo, error) {
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	list, err := k.Dynamic.Resource(gvr).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	var crds []model.CRDInfo

	for _, item := range list.Items {
		info, ok := k.extractCRDInfo(item.Object)
		if ok {
			crds = append(crds, info)
		}
	}

	return crds, nil
}

func (k *Client) extractCRDInfo(obj map[string]any) (model.CRDInfo, bool) {
	name, ok := getString(obj, "metadata", "name")
	if !ok {
		return model.CRDInfo{}, false
	}

	group, _ := getString(obj, "spec", "group")
	plural, _ := getString(obj, "spec", "names", "plural")

	versions, ok := getSlice(obj, "spec", "versions")
	if !ok {
		return model.CRDInfo{}, false
	}

	scope, _ := getString(obj, "spec", "scope")
	namespaced := scope != "Cluster"

	return model.CRDInfo{
		Name:       name,
		Group:      group,
		Version:    k.findPreferredVersion(versions),
		Resource:   plural,
		Namespaced: namespaced,
	}, true
}

func (k *Client) findPreferredVersion(versions []any) string {
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

func (*Client) isServedAndStored(v any) (string, bool) {
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

// ListResources fetches resources for a specific CRD.
func (k *Client) ListResources(crd model.CRDInfo, namespace string) ([]model.ResourceInfo, error) {
	gvr := schema.GroupVersionResource{
		Group:    crd.Group,
		Version:  crd.Version,
		Resource: crd.Resource,
	}

	listClient := k.Dynamic.Resource(gvr)

	var list *unstructured.UnstructuredList

	var err error

	if crd.Namespaced {
		list, err = listClient.Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	} else {
		list, err = listClient.List(context.Background(), metav1.ListOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list resources for %s (namespaced: %v) in namespace %s: %w",
			crd.Name, crd.Namespaced, namespace, err)
	}

	var resources []model.ResourceInfo

	for _, item := range list.Items {
		resources = append(resources, model.ResourceInfo{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			CRD:       crd,
		})
	}

	return resources, nil
}

// FetchResourceYAML fetches a resource and returns its YAML representation.
// Strictly READ-ONLY.
func (k *Client) FetchResourceYAML(crd model.CRDInfo, name, namespace string) (string, error) {
	gvr := schema.GroupVersionResource{
		Group:    crd.Group,
		Version:  crd.Version,
		Resource: crd.Resource,
	}

	obj, err := k.Dynamic.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
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
