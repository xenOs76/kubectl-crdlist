package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

// TestHandleBrowsingKeysEditInCRDList verifies e does not start edit outside YAML view.
func TestHandleBrowsingKeysEditInCRDList(t *testing.T) {
	m := Model{
		State: model.StateCRDList,
		Mode:  model.ModeBrowsing,
		K8s:   &k8s.Client{},
	}

	newModel, cmd := m.handleBrowsingKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	updated := assertModel(t, newModel)

	assert.Nil(t, cmd)
	assert.Equal(t, model.StateCRDList, updated.State)
}

// TestHandleBrowsingKeysEditInYAMLView verifies e starts kubectl edit in YAML view.
func TestHandleBrowsingKeysEditInYAMLView(t *testing.T) {
	tmp := t.TempDir()
	fakeKubectl := filepath.Join(tmp, "kubectl")
	err := os.WriteFile(fakeKubectl, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	require.NoError(t, err)
	t.Setenv("PATH", tmp)

	m := Model{
		State: model.StateYAMLView,
		Mode:  model.ModeBrowsing,
		Ctx:   context.Background(),
		K8s:   &k8s.Client{},
		SelectedCRD: model.CRDInfo{
			Group:      "example.com",
			Resource:   "widgets",
			Namespaced: true,
		},
		SelectedRes: model.ResourceInfo{
			Name:      "my-widget",
			Namespace: "default",
		},
	}

	newModel, cmd := m.handleBrowsingKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	updated := assertModel(t, newModel)

	assert.NotNil(t, cmd)
	assert.NoError(t, updated.Err)
}

// TestClearScreenCmd verifies the clear-screen command returns tea.ClearScreen.
func TestClearScreenCmd(t *testing.T) {
	assert.Equal(t, tea.ClearScreen(), clearScreenCmd()())
}

// firstCmdFromSequence returns the first command from a tea.Sequence/Batch wrapper.
func firstCmdFromSequence(t *testing.T, cmd tea.Cmd) tea.Cmd {
	t.Helper()

	require.NotNil(t, cmd)

	msg := cmd()

	rv := reflect.ValueOf(msg)
	if rv.Kind() == reflect.Slice && rv.Len() > 0 {
		c, ok := rv.Index(0).Interface().(tea.Cmd)
		require.True(t, ok, "expected tea.Cmd in sequence, got %T", rv.Index(0).Interface())

		return c
	}

	return cmd
}

// TestViewUsesAltScreen verifies the TUI renders in the alternate screen buffer.
func TestViewUsesAltScreen(t *testing.T) {
	m := Model{State: model.StateCRDList, Width: 80, Height: 24}

	view := m.View()

	assert.True(t, view.AltScreen)
	assert.NotEmpty(t, view.Content)
}

// TestViewEmptyWhileResumingFromEdit verifies stale content is suppressed after kubectl edit.
func TestViewEmptyWhileResumingFromEdit(t *testing.T) {
	m := Model{
		State:            model.StateYAMLView,
		resumingFromEdit: true,
		SelectedYAML:     "stale",
		Width:            80,
		Height:           24,
	}

	view := m.View()

	assert.True(t, view.AltScreen)
	assert.Empty(t, view.Content)
}

// TestUpdateEditPendingMsg sets the resuming-from-edit guard.
func TestUpdateEditPendingMsg(t *testing.T) {
	m := Model{State: model.StateYAMLView}

	newModel, cmd := m.Update(model.EditPendingMsg{})
	updated := assertModel(t, newModel)

	assert.True(t, updated.resumingFromEdit)
	assert.Nil(t, cmd)
}

// TestStartEditReturnsPendingThenExec verifies edit starts with EditPendingMsg before exec.
func TestStartEditReturnsPendingThenExec(t *testing.T) {
	tmp := t.TempDir()
	fakeKubectl := filepath.Join(tmp, "kubectl")
	err := os.WriteFile(fakeKubectl, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	require.NoError(t, err)
	t.Setenv("PATH", tmp)

	m := Model{
		State: model.StateYAMLView,
		Mode:  model.ModeBrowsing,
		Ctx:   context.Background(),
		K8s:   &k8s.Client{},
		SelectedCRD: model.CRDInfo{
			Group:      "example.com",
			Resource:   "widgets",
			Namespaced: true,
		},
		SelectedRes: model.ResourceInfo{
			Name:      "my-widget",
			Namespace: "default",
		},
	}

	_, cmd := m.startEdit()
	require.NotNil(t, cmd)

	first := firstCmdFromSequence(t, cmd)
	assert.Equal(t, model.EditPendingMsg{}, first())
}

// cmdAtSequenceIndex returns the command at index in a tea.Sequence wrapper.
func cmdAtSequenceIndex(t *testing.T, cmd tea.Cmd, index int) tea.Cmd {
	t.Helper()

	require.NotNil(t, cmd)

	msg := cmd()
	rv := reflect.ValueOf(msg)
	require.Equal(t, reflect.Slice, rv.Kind())
	require.Greater(t, rv.Len(), index)

	c, ok := rv.Index(index).Interface().(tea.Cmd)
	require.True(t, ok, "expected tea.Cmd at index %d, got %T", index, rv.Index(index).Interface())

	return c
}

// TestUpdateEditFinishedMsgSuccess verifies a successful edit clears the screen then refetches YAML.
func TestUpdateEditFinishedMsgSuccess(t *testing.T) {
	m := Model{
		State:        model.StateYAMLView,
		SelectedYAML: "old",
		Width:        80,
		Height:       24,
		K8s: &k8s.Client{
			Dynamic: fake.NewSimpleDynamicClient(runtime.NewScheme()),
		},
		Ctx: context.Background(),
		SelectedCRD: model.CRDInfo{
			Name:       "widgets.example.com",
			Group:      "example.com",
			Version:    "v1",
			Resource:   "widgets",
			Namespaced: true,
		},
		SelectedRes: model.ResourceInfo{
			Name:      "my-widget",
			Namespace: "default",
		},
	}

	newModel, cmd := m.Update(model.EditFinishedMsg{})
	updated := assertModel(t, newModel)

	assert.False(t, updated.resumingFromEdit)
	assert.True(t, updated.Loading)
	require.NotNil(t, cmd)

	assert.Equal(t, tea.ClearScreen(), cmdAtSequenceIndex(t, cmd, 0)())
	assert.Equal(t, tea.WindowSizeMsg{Width: 80, Height: 24}, cmdAtSequenceIndex(t, cmd, 1)())
}

// TestUpdateEditFinishedMsgError verifies a failed edit clears the screen before showing the error.
func TestUpdateEditFinishedMsgError(t *testing.T) {
	m := Model{State: model.StateYAMLView}

	editErr := errors.New("edit failed")
	newModel, cmd := m.Update(model.EditFinishedMsg{Err: editErr})
	updated := assertModel(t, newModel)

	require.ErrorIs(t, updated.Err, editErr)
	assert.False(t, updated.resumingFromEdit)
	assert.False(t, updated.Loading)
	require.NotNil(t, cmd)
	assert.Equal(t, tea.ClearScreen(), cmd())
}
