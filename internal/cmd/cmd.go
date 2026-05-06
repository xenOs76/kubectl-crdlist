package cmd

import (
	"context"
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/xenos76/kubectl-crdlist/internal/k8s"
	"github.com/xenos76/kubectl-crdlist/internal/ui"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var configFlags *genericclioptions.ConfigFlags

// Execute runs the root command and returns an error if any.
func Execute() error {
	configFlags = genericclioptions.NewConfigFlags(true)

	rootCmd := &cobra.Command{
		Use:   "kubectl-crdlist",
		Short: "A TUI for browsing Kubernetes CRDs and their instances",
		RunE: func(cmd *cobra.Command, _ []string) error {
			k, err := k8s.NewClient(configFlags)
			if err != nil {
				return err
			}

			ns, _, err := configFlags.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				ns = "default"
			}

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			m := ui.NewModel(ctx, cancel, k, ns)
			p := tea.NewProgram(m, tea.WithContext(ctx))

			_, err = p.Run()
			if err != nil {
				// Ignore context cancellation errors on exit
				if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled") {
					return nil
				}

				return err
			}

			return nil
		},
	}

	configFlags.AddFlags(rootCmd.Flags())

	return rootCmd.Execute()
}
