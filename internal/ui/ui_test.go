package ui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-crdlist/internal/k8s"
	"github.com/xenos76/kubectl-crdlist/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func assertModel(t *testing.T, m tea.Model) Model {
	t.Helper()

	res, ok := m.(Model)
	require.True(t, ok, "expected Model, got %T", m)

	return res
}

func TestApplyFilterCRDList(t *testing.T) {
	m := Model{
		State: model.StateCRDList,
		Crds: []model.CRDInfo{
			{Group: "networking.k8s.io", Name: "ingressclasses.networking.k8s.io"},
			{Group: "apps", Name: "deployments.apps"},
			{Group: "core", Name: "pods.core"},
		},
		Filter: "deploy",
		Mode:   model.ModeFiltering,
	}

	m.applyFilter()

	require.Len(t, m.FilteredCRDs, 1)
	assert.Equal(t, "deployments.apps", m.FilteredCRDs[0].Name)
}

func TestMoveDownCRD(t *testing.T) {
	m := Model{
		State:           model.StateCRDList,
		FilteredCRDs:    make([]model.CRDInfo, 20),
		CrdCursor:       0,
		CrdScrollOffset: 0,
		Height:          24,
	}

	m.moveDown(5)

	assert.Equal(t, 5, m.CrdCursor)
}

func TestHandleEscape(t *testing.T) {
	m := Model{
		Mode:   model.ModeFiltering,
		Filter: "test",
	}

	m.handleEscape()

	assert.Equal(t, model.ModeBrowsing, m.Mode)
}

func TestViewRendering(t *testing.T) {
	m := Model{
		Width:  80,
		Height: 24,
	}

	m.Err = errors.New("test error")
	out := m.View()
	assert.Contains(t, out.Content, "Error:")
}

func TestUpdateMessages(t *testing.T) {
	m := Model{}
	crdMsg := model.CRDsLoadedMsg{
		{Group: "b", Name: "b-crd"},
		{Group: "a", Name: "a-crd"},
	}
	newModel, _ := m.Update(crdMsg)
	mUpdated := assertModel(t, newModel)

	require.Len(t, mUpdated.Crds, 2)
	assert.Equal(t, "a", mUpdated.Crds[0].Group)
}

func TestFetchCommands(t *testing.T) {
	scheme := runtime.NewScheme()
	testCRD := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]any{"name": "testcrds.example.com"},
			"spec": map[string]any{
				"group": "example.com",
				"names": map[string]any{"plural": "testcrds"},
				"versions": []any{
					map[string]any{"name": "v1", "served": true, "storage": true},
				},
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{
			Group:    "apiextensions.k8s.io",
			Version:  "v1",
			Resource: "customresourcedefinitions",
		}: "CustomResourceDefinitionList",
	}, testCRD)

	k := &k8s.Client{Dynamic: dynClient}
	m := Model{
		K8s: k,
		SelectedCRD: model.CRDInfo{
			Name:     "testcrds.example.com",
			Group:    "example.com",
			Version:  "v1",
			Resource: "testcrds",
		},
	}

	cmd := m.fetchCRDs()
	msg := cmd()
	require.IsType(t, model.CRDsLoadedMsg{}, msg)
}
