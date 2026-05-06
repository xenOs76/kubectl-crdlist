package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-crdlist/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

// TestExtractCRDInfo verifies the parsing logic for Custom Resource Definitions.
func TestExtractCRDInfo(t *testing.T) {
	k := &Client{}

	t.Run("valid CRD", func(t *testing.T) {
		testExtractCRDInfoValid(t, k)
	})

	t.Run("cluster-scoped CRD", func(t *testing.T) {
		testExtractCRDInfoClusterScoped(t, k)
	})

	t.Run("invalid cases", func(t *testing.T) {
		testExtractCRDInfoInvalid(t, k)
	})
}

// testExtractCRDInfoValid verifies extraction from a correctly formatted namespaced CRD.
func testExtractCRDInfoValid(t *testing.T, k *Client) {
	t.Helper()

	validObj := map[string]any{
		"metadata": map[string]any{
			"name": "crds.example.com",
		},
		"spec": map[string]any{
			"group": "example.com",
			"names": map[string]any{
				"plural": "crds",
			},
			"versions": []any{
				map[string]any{
					"name":    "v1",
					"served":  true,
					"storage": true,
				},
			},
		},
	}

	info, ok := k.extractCRDInfo(validObj)
	require.True(t, ok, "expected extraction to succeed")
	assert.Equal(t, "crds.example.com", info.Name)
	assert.Equal(t, "example.com", info.Group)
	assert.Equal(t, "crds", info.Resource)
	assert.Equal(t, "v1", info.Version)
	assert.True(t, info.Namespaced, "expected namespaced to be true by default")
}

// testExtractCRDInfoClusterScoped verifies extraction from a cluster-scoped CRD.
func testExtractCRDInfoClusterScoped(t *testing.T, k *Client) {
	t.Helper()

	clusterObj := map[string]any{
		"metadata": map[string]any{
			"name": "clusters.example.com",
		},
		"spec": map[string]any{
			"group": "example.com",
			"scope": "Cluster",
			"names": map[string]any{
				"plural": "clusters",
			},
			"versions": []any{
				map[string]any{
					"name":    "v1",
					"served":  true,
					"storage": true,
				},
			},
		},
	}

	info, ok := k.extractCRDInfo(clusterObj)
	require.True(t, ok, "expected extraction to succeed")
	assert.False(t, info.Namespaced, "expected namespaced to be false for Cluster scope")
}

// testExtractCRDInfoInvalid verifies that extraction fails correctly for malformed inputs.
func testExtractCRDInfoInvalid(t *testing.T, k *Client) {
	t.Helper()

	cases := []struct {
		name string
		obj  map[string]any
	}{
		{"missing metadata", map[string]any{}},
		{"missing name", map[string]any{"metadata": map[string]any{}}},
		{"missing spec", map[string]any{"metadata": map[string]any{"name": "something"}}},
		{"missing versions", map[string]any{
			"metadata": map[string]any{"name": "something"},
			"spec":     map[string]any{"group": "test", "names": map[string]any{"plural": "tests"}},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := k.extractCRDInfo(tc.obj)
			assert.False(t, ok, "expected failure for %s", tc.name)
		})
	}
}

// TestFindPreferredVersion verifies the logic for selecting the best API version.
func TestFindPreferredVersion(t *testing.T) {
	k := &Client{}

	versions := []any{
		map[string]any{"name": "v1alpha1", "served": true, "storage": false},
		map[string]any{"name": "v1", "served": true, "storage": true},
	}

	version := k.findPreferredVersion(versions)
	assert.Equal(t, "v1", version)

	// Test fallback to first if none match both
	versionsFallback := []any{
		map[string]any{"name": "v1beta1", "served": false, "storage": false},
	}

	versionFallback := k.findPreferredVersion(versionsFallback)
	assert.Equal(t, "v1beta1", versionFallback)

	// Test totally invalid version map
	versionEmpty := k.findPreferredVersion([]any{"invalid"})
	assert.Empty(t, versionEmpty)
}

// TestIsServedAndStored verifies the validation of API version status.
func TestIsServedAndStored(t *testing.T) {
	k := &Client{}

	// Valid
	v1 := map[string]any{"name": "v1", "served": true, "storage": true}
	name, ok := k.isServedAndStored(v1)
	assert.True(t, ok)
	assert.Equal(t, "v1", name)

	// Not storage
	v2 := map[string]any{"name": "v2", "served": true, "storage": false}
	_, ok = k.isServedAndStored(v2)
	assert.False(t, ok)

	// Not served
	v3 := map[string]any{"name": "v3", "served": false, "storage": true}
	_, ok = k.isServedAndStored(v3)
	assert.False(t, ok)

	// Invalid types
	v4 := map[string]any{"name": "v4", "served": "yes", "storage": 1}
	_, ok = k.isServedAndStored(v4)
	assert.False(t, ok)
}

// TestListCRDs verifies that the client correctly lists CRDs from the cluster.
func TestListCRDs(t *testing.T) {
	scheme := runtime.NewScheme()

	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	testCRD := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]any{
				"name": "testcrds.example.com",
			},
			"spec": map[string]any{
				"group": "example.com",
				"names": map[string]any{
					"plural": "testcrds",
				},
				"versions": []any{
					map[string]any{
						"name":    "v1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		crdGVR: "CustomResourceDefinitionList",
	}, testCRD)

	k := &Client{
		Dynamic: dynClient,
	}

	crds, err := k.ListCRDs(context.Background())
	require.NoError(t, err)
	require.Len(t, crds, 1)
	assert.Equal(t, "testcrds.example.com", crds[0].Name)
	assert.Equal(t, "testcrds", crds[0].Resource)
}

// TestListResourcesAndYAML verifies both resource listing and YAML fetching.
func TestListResourcesAndYAML(t *testing.T) {
	scheme := runtime.NewScheme()

	crd := model.CRDInfo{
		Name:       "testcrds.example.com",
		Group:      "example.com",
		Version:    "v1",
		Resource:   "testcrds",
		Namespaced: true,
	}

	testResGVR := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "testcrds",
	}

	testResource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestCrd",
			"metadata": map[string]any{
				"name":      "my-test-res",
				"namespace": "default",
			},
			"spec": map[string]any{
				"hello": "world",
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		testResGVR: "TestCrdList",
	}, testResource)

	k := &Client{
		Dynamic: dynClient,
	}

	// Test List resources
	resources, err := k.ListResources(context.Background(), crd, "default")
	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "my-test-res", resources[0].Name)
	assert.Equal(t, "default", resources[0].Namespace)

	// Test YAML fetch
	yamlData, err := k.FetchResourceYAML(context.Background(), crd, "my-test-res", "default")
	require.NoError(t, err)
	assert.Contains(t, yamlData, "hello: world")
}

// TestListResourcesClusterScoped verifies listing for cluster-wide resources.
func TestListResourcesClusterScoped(t *testing.T) {
	scheme := runtime.NewScheme()

	crd := model.CRDInfo{
		Name:       "clusters.example.com",
		Group:      "example.com",
		Version:    "v1",
		Resource:   "clusters",
		Namespaced: false,
	}

	testResource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Cluster",
			"metadata": map[string]any{
				"name": "my-cluster-res",
			},
		},
	}

	testResGVR := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "clusters",
	}

	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		testResGVR: "ClusterList",
	}, testResource)

	k := &Client{
		Dynamic: dynClient,
	}

	// Test List resources - should work even if namespace is provided but ignored
	resources, err := k.ListResources(context.Background(), crd, "some-namespace")
	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "my-cluster-res", resources[0].Name)
}

// TestInitK8sClient verifies the manual initialization of the client struct.
func TestInitK8sClient(t *testing.T) {
	t.Run("basic initialization", func(t *testing.T) {
		scheme := runtime.NewScheme()
		dynamicClient := fake.NewSimpleDynamicClient(scheme)

		k := &Client{
			Dynamic: dynamicClient,
		}

		require.NotNil(t, k)
		assert.NotNil(t, k.Dynamic)
	})
}

// TestExtractCRDInfoEdgeCases verifies extraction logic across various edge cases and malformed inputs.
func TestExtractCRDInfoEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]any
		wantOk   bool
		wantName string
	}{
		{
			name: "valid CRD info",
			obj: map[string]any{
				"metadata": map[string]any{
					"name": "testcrds.example.com",
				},
				"spec": map[string]any{
					"group": "example.com",
					"names": map[string]any{
						"plural": "testcrds",
					},
					"versions": []any{
						map[string]any{
							"name":    "v1",
							"served":  true,
							"storage": true,
						},
					},
				},
			},
			wantOk:   true,
			wantName: "testcrds.example.com",
		},
		{
			name:   "missing metadata",
			obj:    map[string]any{},
			wantOk: false,
		},
		{
			name: "missing spec",
			obj: map[string]any{
				"metadata": map[string]any{
					"name": "test",
				},
			},
			wantOk: false,
		},
		{
			name: "missing names",
			obj: map[string]any{
				"metadata": map[string]any{
					"name": "test",
				},
				"spec": map[string]any{
					"group": "example.com",
				},
			},
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &Client{}

			info, ok := k.extractCRDInfo(tt.obj)

			assert.Equal(t, tt.wantOk, ok)

			if ok {
				assert.Equal(t, tt.wantName, info.Name)
			}
		})
	}
}
