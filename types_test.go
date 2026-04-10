package main

import (
	"testing"
)

func TestInitialModel(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		wantState   viewState
		wantLoading bool
		wantNS      string
	}{
		{
			name:        "default namespace",
			namespace:   "default",
			wantState:   stateCRDList,
			wantLoading: true,
			wantNS:      "default",
		},
		{
			name:        "custom namespace",
			namespace:   "kube-system",
			wantState:   stateCRDList,
			wantLoading: true,
			wantNS:      "kube-system",
		},
		{
			name:        "empty namespace",
			namespace:   "",
			wantState:   stateCRDList,
			wantLoading: true,
			wantNS:      "",
		},
		{
			name:        "namespace with dash",
			namespace:   "my-namespace",
			wantState:   stateCRDList,
			wantLoading: true,
			wantNS:      "my-namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock k8sClient for testing
			mockClient := &k8sClient{}

			got := initialModel(mockClient, tt.namespace)

			if got.state != tt.wantState {
				t.Errorf("state = %v, want %v", got.state, tt.wantState)
			}

			if got.loading != tt.wantLoading {
				t.Errorf("loading = %v, want %v", got.loading, tt.wantLoading)
			}

			if got.currentNamespace != tt.wantNS {
				t.Errorf("currentNamespace = %q, want %q", got.currentNamespace, tt.wantNS)
			}

			if got.allNamespaces != false {
				t.Errorf("allNamespaces = %v, want false", got.allNamespaces)
			}

			if got.k8s != mockClient {
				t.Errorf("k8s client not set correctly")
			}
		})
	}
}
