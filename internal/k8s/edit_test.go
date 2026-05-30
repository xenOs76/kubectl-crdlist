package k8s

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-crdlist/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestValidateEditIdentityUnchanged(t *testing.T) {
	session := &EditSession{
		origAPIVersion: "example.com/v1",
		origKind:       "Widget",
		origName:       "how-i-did-it",
	}

	edited := map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata": map[string]any{
			"name": "how-i-did-it",
		},
		"spec": map[string]any{"note": "updated"},
	}

	require.NoError(t, validateEditIdentity(session, edited))
}

func TestValidateEditIdentityRejectsChanges(t *testing.T) {
	session := &EditSession{
		origAPIVersion: "example.com/v1",
		origKind:       "Widget",
		origName:       "how-i-did-it",
	}

	edited := map[string]any{
		"apiVersion": "example.com/v2",
		"kind":       "Widget",
		"metadata": map[string]any{
			"name": "renamed",
		},
	}

	err := validateEditIdentity(session, edited)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiVersion")
	assert.Contains(t, err.Error(), "metadata.name")
	assert.Contains(t, err.Error(), "create a new resource")
	assert.Contains(t, err.Error(), "delete \"how-i-did-it\"")
}

func TestEditorCommandUsesKubeEditor(t *testing.T) {
	t.Setenv("KUBE_EDITOR", "nano")
	t.Setenv("SHELL", "/bin/sh")

	cmd, err := editorCommand(context.Background(), "/tmp/example.yaml")
	require.NoError(t, err)
	assert.Equal(t, "/bin/sh", cmd.Path)
	assert.Equal(t, []string{"-c", `nano "/tmp/example.yaml"`}, cmd.Args[1:])
}

func TestBeginResourceEditWritesManifest(t *testing.T) {
	crdGVR := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "widgets",
	}

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":            "how-i-did-it",
				"namespace":       "default",
				"resourceVersion": "42",
			},
			"spec": map[string]any{"title": "original"},
		},
	}

	scheme := runtime.NewScheme()
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		crdGVR: "WidgetList",
	}, obj)

	k := &Client{Dynamic: dyn}
	crd := model.CRDInfo{
		Group:      "example.com",
		Version:    "v1",
		Resource:   "widgets",
		Namespaced: true,
	}
	res := model.ResourceInfo{Name: "how-i-did-it", Namespace: "default"}

	t.Setenv("KUBE_EDITOR", "true")
	t.Setenv("SHELL", "/bin/sh")

	session, cmd, err := k.BeginResourceEdit(context.Background(), crd, res)
	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotNil(t, cmd)

	data, err := os.ReadFile(session.tempFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "how-i-did-it")
	assert.Contains(t, string(data), "example.com/v1")

	session.Cleanup()
}

func TestCompleteResourceEditNilSession(t *testing.T) {
	k := &Client{}
	err := k.CompleteResourceEdit(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil EditSession")
}

func TestCompleteResourceEditUpdatesResource(t *testing.T) {
	crdGVR := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "widgets",
	}

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]any{
				"name":            "how-i-did-it",
				"namespace":       "default",
				"resourceVersion": "42",
			},
			"spec": map[string]any{"title": "original"},
		},
	}

	scheme := runtime.NewScheme()
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		crdGVR: "WidgetList",
	}, obj)

	k := &Client{Dynamic: dyn}

	tempDir := t.TempDir()
	tempPath := filepath.Join(tempDir, "edit.yaml")
	manifest := `apiVersion: example.com/v1
kind: Widget
metadata:
  name: how-i-did-it
  namespace: default
  resourceVersion: "42"
spec:
  title: updated
`
	require.NoError(t, os.WriteFile(tempPath, []byte(manifest), 0o600))

	session := &EditSession{
		tempFile:       tempPath,
		origAPIVersion: "example.com/v1",
		origKind:       "Widget",
		origName:       "how-i-did-it",
		gvr:            crdGVR,
		namespace:      "default",
		namespaced:     true,
	}

	err := k.CompleteResourceEdit(context.Background(), session)
	require.NoError(t, err)

	updated, err := dyn.Resource(crdGVR).Namespace("default").Get(
		context.Background(),
		"how-i-did-it",
		metav1.GetOptions{},
	)
	require.NoError(t, err)

	title, _, err := unstructured.NestedString(updated.Object, "spec", "title")
	require.NoError(t, err)
	assert.Equal(t, "updated", title)
}
