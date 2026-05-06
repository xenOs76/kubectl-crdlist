package cmd

import (
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
		RunE: func(_ *cobra.Command, _ []string) error {
			k, err := k8s.NewClient(configFlags)
			if err != nil {
				return err
			}

			ns, _, err := configFlags.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				ns = "default"
			}

			m := ui.NewModel(k, ns)
			p := tea.NewProgram(m)

			_, err = p.Run()

			return err
		},
	}

	configFlags.AddFlags(rootCmd.Flags())

	return rootCmd.Execute()
}
