package main

import (
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestConfigFlagsInitialization(t *testing.T) {
	// Test that configFlags is properly initialized
	flags := genericclioptions.NewConfigFlags(true)

	if flags == nil {
		t.Fatal("NewConfigFlags returned nil")
	}

	if flags.Namespace == nil {
		t.Error("Namespace flag not initialized")
	}

	if flags.Context == nil {
		t.Error("Context flag not initialized")
	}

	if flags.ClusterName == nil {
		t.Error("ClusterName flag not initialized")
	}
}

func TestConfigFlagsNamespaceFlag(t *testing.T) {
	tests := []struct {
		name    string
		ns      string
		wantErr bool
	}{
		{
			name:    "default namespace",
			ns:      "default",
			wantErr: false,
		},
		{
			name:    "custom namespace",
			ns:      "kube-system",
			wantErr: false,
		},
		{
			name:    "empty namespace",
			ns:      "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := genericclioptions.NewConfigFlags(true)
			flags.Namespace = &tt.ns

			if flags.Namespace == nil {
				t.Error("Namespace not set")
			}

			if *flags.Namespace != tt.ns {
				t.Errorf("Namespace = %q, want %q", *flags.Namespace, tt.ns)
			}
		})
	}
}
