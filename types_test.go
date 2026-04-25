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
		{"default namespace", "default", stateCRDList, true, "default"},
		{"custom namespace", "kube-system", stateCRDList, true, "kube-system"},
		{"empty namespace", "", stateCRDList, true, ""},
		{"namespace with dash", "my-namespace", stateCRDList, true, "my-namespace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &k8sClient{}
			got := initialModel(mockClient, tt.namespace)
			verifyInitialModel(t, &got, mockClient, expectedModel{tt.wantState, tt.wantLoading, tt.wantNS})
		})
	}
}

type expectedModel struct {
	state   viewState
	loading bool
	ns      string
}

func verifyInitialModel(t *testing.T, got *model, mockClient *k8sClient, want expectedModel) {
	t.Helper()

	if got.state != want.state {
		t.Errorf("state = %v, want %v", got.state, want.state)
	}

	if got.loading != want.loading {
		t.Errorf("loading = %v, want %v", got.loading, want.loading)
	}

	if got.currentNamespace != want.ns {
		t.Errorf("currentNamespace = %q, want %q", got.currentNamespace, want.ns)
	}

	if got.allNamespaces {
		t.Errorf("allNamespaces = %v, want false", got.allNamespaces)
	}

	if got.k8s != mockClient {
		t.Error("k8s client not set correctly")
	}
}
