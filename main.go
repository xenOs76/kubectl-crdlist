package main

import (
	"fmt"
	"os"

	"github.com/xenos76/kubectl-crdlist/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		os.Exit(1)
	}
}
