package k8s

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/xenos76/kubectl-crdlist/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// EditSession tracks an in-progress resource edit backed by a temporary manifest file.
type EditSession struct {
	tempFile       string
	origAPIVersion string
	origKind       string
	origName       string
	gvr            schema.GroupVersionResource
	namespace      string
	namespaced     bool
}

// Cleanup removes the temporary edit file.
func (s *EditSession) Cleanup() {
	if s.tempFile != "" {
		_ = os.Remove(s.tempFile)
		s.tempFile = ""
	}
}

// BeginResourceEdit fetches the resource, writes a YAML manifest for editing, and returns
// a command that launches the user's editor (KUBE_EDITOR, EDITOR, VISUAL, or vi).
func (k *Client) BeginResourceEdit(
	ctx context.Context,
	crd model.CRDInfo,
	res model.ResourceInfo,
) (*EditSession, *exec.Cmd, error) {
	obj, err := k.getResource(ctx, crd, res.Name, res.Namespace)
	if err != nil {
		return nil, nil, err
	}

	identity, err := readObjectIdentity(obj.Object)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid resource manifest: %w", err)
	}

	// Match YAML view output: drop managed fields for a cleaner editor buffer.
	obj.SetManagedFields(nil)

	manifest, err := yaml.Marshal(obj.Object)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal resource for edit: %w", err)
	}

	tempFile, err := os.CreateTemp("", "kubectl-crdlist-edit-*.yaml")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create edit temp file: %w", err)
	}

	tempPath := tempFile.Name()

	if _, err := tempFile.Write(manifest); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)

		return nil, nil, fmt.Errorf("failed to write edit temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)

		return nil, nil, fmt.Errorf("failed to close edit temp file: %w", err)
	}

	session := &EditSession{
		tempFile:       tempPath,
		origAPIVersion: identity.apiVersion,
		origKind:       identity.kind,
		origName:       identity.name,
		gvr: schema.GroupVersionResource{
			Group:    crd.Group,
			Version:  crd.Version,
			Resource: crd.Resource,
		},
		namespace:  res.Namespace,
		namespaced: crd.Namespaced,
	}

	cmd, err := editorCommand(context.WithoutCancel(ctx), tempPath)
	if err != nil {
		session.Cleanup()

		return nil, nil, err
	}

	return session, cmd, nil
}

// CompleteResourceEdit reads the edited manifest and updates the resource in the cluster.
func (k *Client) CompleteResourceEdit(ctx context.Context, session *EditSession) error {
	if session == nil {
		return errors.New("CompleteResourceEdit: nil EditSession")
	}

	defer session.Cleanup()

	edited, err := os.ReadFile(session.tempFile)
	if err != nil {
		return fmt.Errorf("failed to read edited manifest: %w", err)
	}

	var parsed map[string]any

	if err := yaml.Unmarshal(edited, &parsed); err != nil {
		return fmt.Errorf("failed to parse edited manifest: %w", err)
	}

	if err := validateEditIdentity(session, parsed); err != nil {
		return err
	}

	updated := &unstructured.Unstructured{Object: parsed}

	client := k.Dynamic.Resource(session.gvr)
	if session.namespaced {
		_, err = client.Namespace(session.namespace).Update(ctx, updated, metav1.UpdateOptions{})
	} else {
		_, err = client.Update(ctx, updated, metav1.UpdateOptions{})
	}

	if err != nil {
		return fmt.Errorf("failed to apply edited resource: %w", err)
	}

	return nil
}

type objectIdentity struct {
	apiVersion string
	kind       string
	name       string
}

func readObjectIdentity(obj map[string]any) (objectIdentity, error) {
	apiVersion, ok := obj["apiVersion"].(string)
	if !ok || apiVersion == "" {
		return objectIdentity{}, errors.New("missing apiVersion")
	}

	kind, ok := obj["kind"].(string)
	if !ok || kind == "" {
		return objectIdentity{}, errors.New("missing kind")
	}

	name, ok, err := unstructured.NestedString(obj, "metadata", "name")
	if err != nil {
		return objectIdentity{}, fmt.Errorf("metadata.name: %w", err)
	}

	if !ok || name == "" {
		return objectIdentity{}, errors.New("missing metadata.name")
	}

	return objectIdentity{apiVersion: apiVersion, kind: kind, name: name}, nil
}

func validateEditIdentity(session *EditSession, edited map[string]any) error {
	identity, err := readObjectIdentity(edited)
	if err != nil {
		return fmt.Errorf("edited manifest is invalid: %w", err)
	}

	var changed []string

	if identity.apiVersion != session.origAPIVersion {
		changed = append(changed, fmt.Sprintf("apiVersion (%q → %q)", session.origAPIVersion, identity.apiVersion))
	}

	if identity.kind != session.origKind {
		changed = append(changed, fmt.Sprintf("kind (%q → %q)", session.origKind, identity.kind))
	}

	if identity.name != session.origName {
		changed = append(changed, fmt.Sprintf("metadata.name (%q → %q)", session.origName, identity.name))
	}

	if len(changed) == 0 {
		return nil
	}

	return fmt.Errorf(
		"cannot change resource identity (%s); only spec and other fields may be edited\n\n"+
			"To rename or change apiVersion/kind: create a new resource with the desired identity, "+
			"then delete %q",
		strings.Join(changed, ", "),
		session.origName,
	)
}

func editorCommand(ctx context.Context, path string) (*exec.Cmd, error) {
	editor := os.Getenv("KUBE_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}

	if editor == "" {
		editor = os.Getenv("VISUAL")
	}

	if editor == "" {
		editor = "vi"
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	script := strings.TrimSpace(editor) + " " + strconv.Quote(path)

	return exec.CommandContext(ctx, shell, "-c", script), nil
}
