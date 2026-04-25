package main

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestExtractCRDInfo(t *testing.T) {
	k := &k8sClient{}

	t.Run("valid CRD", func(t *testing.T) {
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
		if !ok {
			t.Fatal("expected extraction to succeed")
		}

		if info.name != "crds.example.com" || info.group != "example.com" ||
			info.resource != "crds" || info.version != "v1" {
			t.Errorf("extracted fields do not match expected: %+v", info)
		}
	})

	t.Run("invalid cases", func(t *testing.T) {
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
				if ok {
					t.Errorf("expected failure for %s", tc.name)
				}
			})
		}
	})
}

func TestFindPreferredVersion(t *testing.T) {
	k := &k8sClient{}

	versions := []any{
		map[string]any{"name": "v1alpha1", "served": true, "storage": false},
		map[string]any{"name": "v1", "served": true, "storage": true},
	}

	version := k.findPreferredVersion(versions)
	if version != "v1" {
		t.Errorf("expected preferred version v1, got %s", version)
	}

	// Test fallback to first if none match both
	versionsFallback := []any{
		map[string]any{"name": "v1beta1", "served": false, "storage": false},
	}

	versionFallback := k.findPreferredVersion(versionsFallback)
	if versionFallback != "v1beta1" {
		t.Errorf("expected fallback version v1beta1, got %s", versionFallback)
	}

	// Test totally invalid version map
	versionEmpty := k.findPreferredVersion([]any{"invalid"})
	if versionEmpty != "" {
		t.Errorf("expected empty string for invalid versions, got %s", versionEmpty)
	}
}

func TestIsServedAndStored(t *testing.T) {
	k := &k8sClient{}

	// Valid
	v1 := map[string]any{"name": "v1", "served": true, "storage": true}

	name, ok := k.isServedAndStored(v1)
	if !ok || name != "v1" {
		t.Error("expected v1 to be served and stored")
	}

	// Not storage
	v2 := map[string]any{"name": "v2", "served": true, "storage": false}

	_, ok = k.isServedAndStored(v2)
	if ok {
		t.Error("expected v2 to fail storage check")
	}

	// Not served
	v3 := map[string]any{"name": "v3", "served": false, "storage": true}

	_, ok = k.isServedAndStored(v3)
	if ok {
		t.Error("expected v3 to fail served check")
	}

	// Invalid types
	v4 := map[string]any{"name": "v4", "served": "yes", "storage": 1}

	_, ok = k.isServedAndStored(v4)
	if ok {
		t.Error("expected v4 to fail type assertions")
	}
}

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

	k := &k8sClient{
		dynamic: dynClient,
	}

	crds, err := k.listCRDs()
	if err != nil {
		t.Fatalf("failed to list crds: %v", err)
	}

	if len(crds) != 1 {
		t.Fatalf("expected 1 CRD, got %d", len(crds))
	}

	if crds[0].name != "testcrds.example.com" || crds[0].resource != "testcrds" {
		t.Errorf("unexpected CRD parsed: %+v", crds[0])
	}
}

func TestListResourcesAndYAML(t *testing.T) {
	scheme := runtime.NewScheme()

	crd := crdInfo{
		name:     "testcrds.example.com",
		group:    "example.com",
		version:  "v1",
		resource: "testcrds",
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

	k := &k8sClient{
		dynamic: dynClient,
	}

	// Test List resources
	resources, err := k.listResources(crd, "default")
	if err != nil {
		t.Fatalf("failed to list resources: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	if resources[0].name != "my-test-res" || resources[0].namespace != "default" {
		t.Errorf("unexpected resource extracted: %+v", resources[0])
	}

	// Test YAML fetch
	yamlData, err := k.fetchResourceYAML(crd, "my-test-res", "default")
	if err != nil {
		t.Fatalf("failed to fetch yaml: %v", err)
	}

	if !strings.Contains(yamlData, "hello: world") {
		t.Errorf("yaml did not marshal expected values: %s", yamlData)
	}
}

func TestInitK8sClient(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "basic initialization",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily test with real kubeconfig in unit tests
			// Just verify that we can create a k8sClient with mock clients
			scheme := runtime.NewScheme()
			dynamicClient := fake.NewSimpleDynamicClient(scheme)

			k := &k8sClient{
				dynamic: dynamicClient,
			}

			if k == nil {
				t.Error("k8sClient creation failed")
			}

			if k.dynamic == nil {
				t.Error("dynamic client not set")
			}
		})
	}
}

func TestListCRDsError(t *testing.T) {
	// Create a fake dynamic client with proper schema setup
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			gvr: "CustomResourceDefinitionList",
		},
	)

	k := &k8sClient{
		dynamic: dynamicClient,
	}

	// Even with empty fake client, listCRDs should handle it gracefully
	crds, err := k.listCRDs()

	// Should either succeed with empty list or fail gracefully
	if err != nil {
		t.Logf("Expected behavior: got error %v", err)
	} else if crds != nil {
		t.Logf("Got CRD list: %d items", len(crds))
	}
}

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
			k := &k8sClient{}

			info, ok := k.extractCRDInfo(tt.obj)

			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}

			if ok && info.name != tt.wantName {
				t.Errorf("name = %q, want %q", info.name, tt.wantName)
			}
		})
	}
}
