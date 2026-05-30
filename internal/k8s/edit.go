package k8s

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/xenos76/kubectl-crdlist/internal/model"
)

// BuildKubectlEditArgs returns kubectl arguments for editing a CR instance.
func (k *Client) BuildKubectlEditArgs(crd model.CRDInfo, res model.ResourceInfo) []string {
	resourceType := fmt.Sprintf("%s.%s", crd.Resource, crd.Group)
	args := []string{"edit", resourceType, res.Name}

	if crd.Namespaced && res.Namespace != "" {
		args = append(args, "-n", res.Namespace)
	}

	args = append(args, k.configFlagArgs(crd, res)...)

	return args
}

func (k *Client) configFlagArgs(crd model.CRDInfo, res model.ResourceInfo) []string {
	if k.ConfigFlags == nil {
		return nil
	}

	f := k.ConfigFlags

	var args []string

	appendStringFlag := func(flag string, value *string) {
		if value != nil && *value != "" {
			args = append(args, flag, *value)
		}
	}

	appendStringFlag("--kubeconfig", f.KubeConfig)
	appendStringFlag("--context", f.Context)
	appendStringFlag("--cluster", f.ClusterName)
	appendStringFlag("--user", f.AuthInfoName)
	appendStringFlag("--server", f.APIServer)
	appendStringFlag("--tls-server-name", f.TLSServerName)
	appendStringFlag("--certificate-authority", f.CAFile)
	appendStringFlag("--client-certificate", f.CertFile)
	appendStringFlag("--client-key", f.KeyFile)
	appendStringFlag("--token", f.BearerToken)
	appendStringFlag("--request-timeout", f.Timeout)

	if f.Insecure != nil && *f.Insecure {
		args = append(args, "--insecure-skip-tls-verify=true")
	}

	if f.DisableCompression != nil && *f.DisableCompression {
		args = append(args, "--disable-compression=true")
	}

	// Use per-resource -n for namespaced edits; only pass global namespace otherwise.
	if f.Namespace != nil && *f.Namespace != "" && (!crd.Namespaced || res.Namespace == "") {
		args = append(args, "-n", *f.Namespace)
	}

	return args
}

// KubectlEditCommand builds an exec.Cmd that runs kubectl edit for the given resource.
func (k *Client) KubectlEditCommand(ctx context.Context, crd model.CRDInfo, res model.ResourceInfo) (*exec.Cmd, error) {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return nil, errors.New("kubectl not found in PATH")
	}

	args := k.BuildKubectlEditArgs(crd, res)
	cmd := exec.CommandContext(ctx, kubectlPath, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd, nil
}
