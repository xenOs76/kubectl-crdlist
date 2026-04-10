package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var configFlags *genericclioptions.ConfigFlags

func main() {
	configFlags = genericclioptions.NewConfigFlags(true)

	rootCmd := &cobra.Command{
		Use:   "kubectl-crdlist",
		Short: "A TUI for browsing Kubernetes CRDs and their instances",
		RunE: func(_ *cobra.Command, _ []string) error {
			k, err := initK8sClient(configFlags)
			if err != nil {
				return err
			}

			ns, _, err := configFlags.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				ns = "default"
			}

			m := initialModel(k, ns)
			p := tea.NewProgram(m)

			_, err = p.Run()

			return err
		},
	}

	configFlags.AddFlags(rootCmd.Flags())

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
