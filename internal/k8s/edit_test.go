package k8s

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-crdlist/internal/model"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestBuildKubectlEditArgsNamespaced(t *testing.T) {
	k := &Client{}

	crd := model.CRDInfo{
		Group:      "example.com",
		Resource:   "widgets",
		Namespaced: true,
	}
	res := model.ResourceInfo{Name: "my-widget", Namespace: "default"}

	args := k.BuildKubectlEditArgs(crd, res)

	assert.Equal(t, []string{"edit", "widgets.example.com", "my-widget", "-n", "default"}, args)
}

func TestBuildKubectlEditArgsClusterScoped(t *testing.T) {
	k := &Client{}

	crd := model.CRDInfo{
		Group:      "example.com",
		Resource:   "clusters",
		Namespaced: false,
	}
	res := model.ResourceInfo{Name: "my-cluster"}

	args := k.BuildKubectlEditArgs(crd, res)

	assert.Equal(t, []string{"edit", "clusters.example.com", "my-cluster"}, args)
}

func TestBuildKubectlEditArgsConfigFlags(t *testing.T) {
	contextName := "foo"
	flags := genericclioptions.NewConfigFlags(true)
	flags.Context = &contextName

	k := &Client{ConfigFlags: flags}

	crd := model.CRDInfo{
		Group:      "example.com",
		Resource:   "widgets",
		Namespaced: true,
	}
	res := model.ResourceInfo{Name: "my-widget", Namespace: "default"}

	args := k.BuildKubectlEditArgs(crd, res)

	assert.Contains(t, args, "--context")
	assert.Contains(t, args, "foo")
	assert.Contains(t, args, "-n")
	assert.Contains(t, args, "default")
}

func TestBuildKubectlEditArgsGlobalNamespaceWhenNotNamespaced(t *testing.T) {
	globalNS := "kube-system"
	flags := genericclioptions.NewConfigFlags(true)
	flags.Namespace = &globalNS

	k := &Client{ConfigFlags: flags}

	crd := model.CRDInfo{
		Group:      "example.com",
		Resource:   "clusters",
		Namespaced: false,
	}
	res := model.ResourceInfo{Name: "my-cluster"}

	args := k.BuildKubectlEditArgs(crd, res)

	assert.Contains(t, args, "-n")
	assert.Contains(t, args, "kube-system")
}

func TestKubectlEditCommand(t *testing.T) {
	k := &Client{}

	crd := model.CRDInfo{
		Group:      "example.com",
		Resource:   "widgets",
		Namespaced: true,
	}
	res := model.ResourceInfo{Name: "my-widget", Namespace: "default"}

	cmd, err := k.KubectlEditCommand(context.Background(), crd, res)
	if err != nil {
		t.Skip("kubectl not found in PATH")
	}

	require.NotNil(t, cmd)
	assert.Equal(t, "kubectl", filepath.Base(cmd.Path))
	assert.Contains(t, cmd.Args, "edit")
	assert.Contains(t, cmd.Args, "widgets.example.com")
	assert.Contains(t, cmd.Args, "my-widget")
}

func TestKubectlEditCommandMissingKubectl(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	k := &Client{}
	crd := model.CRDInfo{Group: "g", Resource: "r"}
	res := model.ResourceInfo{Name: "n"}

	_, err := k.KubectlEditCommand(context.Background(), crd, res)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubectl not found in PATH")
}

func TestKubectlEditCommandWithFakeKubectl(t *testing.T) {
	tmp := t.TempDir()
	fakeKubectl := filepath.Join(tmp, "kubectl")
	err := os.WriteFile(fakeKubectl, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	require.NoError(t, err)

	t.Setenv("PATH", tmp)

	k := &Client{}
	crd := model.CRDInfo{Group: "example.com", Resource: "widgets", Namespaced: true}
	res := model.ResourceInfo{Name: "w", Namespace: "ns"}

	cmd, err := k.KubectlEditCommand(context.Background(), crd, res)
	require.NoError(t, err)
	assert.Equal(t, fakeKubectl, cmd.Path)
}
