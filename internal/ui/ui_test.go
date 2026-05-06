package ui

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-crdlist/internal/k8s"
	"github.com/xenos76/kubectl-crdlist/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

// assertModel is a test helper that asserts a tea.Model is a Model.
func assertModel(t *testing.T, m tea.Model) Model {
	t.Helper()

	res, ok := m.(Model)
	require.True(t, ok, "expected Model, got %T", m)

	return res
}

// TestApplyFilterCRDList verifies that the CRD list is correctly filtered.
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

// TestMoveDownCRD verifies that the cursor moves down correctly in the CRD list.
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

// TestHandleEscape verifies that the escape key correctly exits filtering mode.
func TestHandleEscape(t *testing.T) {
	m := Model{
		Mode:   model.ModeFiltering,
		Filter: "test",
	}

	m.handleEscape()

	assert.Equal(t, model.ModeBrowsing, m.Mode)
}

// TestViewRendering verifies that the view renders correctly when an error is present.
func TestViewRendering(t *testing.T) {
	m := Model{
		Width:  80,
		Height: 24,
	}

	m.Err = errors.New("test error")
	out := m.View()
	assert.Contains(t, out.Content, "Error:")
}

// TestHandleEnterCRDToResource verifies the transition from CRD list to Resource list.
func TestHandleEnterCRDToResource(t *testing.T) {
	scheme := runtime.NewScheme()
	objects := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "test-deploy",
					"namespace": "default",
				},
			},
		},
	}
	client := fake.NewSimpleDynamicClient(scheme, objects...)

	m := Model{
		State: model.StateCRDList,
		FilteredCRDs: []model.CRDInfo{
			{Name: "deployments.apps", Group: "apps", Version: "v1", Resource: "deployments", Namespaced: true},
		},
		CrdCursor: 0,
		K8s: &k8s.Client{
			Dynamic: client,
		},
		Ctx: context.Background(),
	}

	newModel, cmd := m.handleEnter()
	updatedModel := assertModel(t, newModel)

	assert.Equal(t, model.StateResourceList, updatedModel.State)
	assert.NotNil(t, cmd)
}

// TestPerViewFilter verifies that filters are preserved when navigating between views.
func TestPerViewFilter(t *testing.T) {
	m := NewModel(context.Background(), nil, nil, "default")
	m.Filter = "pod"
	m.State = model.StateCRDList

	// Move to resources
	m.saveFilter()
	assert.Equal(t, "pod", m.CRDFilter)

	m.State = model.StateResourceList
	m.loadFilter()
	assert.Empty(t, m.Filter) // Resource filter is initially empty

	m.Filter = "test"
	m.saveFilter()
	assert.Equal(t, "test", m.ResourceFilter)

	// Move back to CRD list
	m.State = model.StateCRDList
	m.loadFilter()
	assert.Equal(t, "pod", m.Filter) // CRD filter is restored
}
