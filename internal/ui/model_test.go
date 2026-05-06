package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xenos76/kubectl-crdlist/internal/k8s"
	"github.com/xenos76/kubectl-crdlist/internal/model"
)

func TestNewModel(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		wantState   model.ViewState
		wantLoading bool
		wantNS      string
	}{
		{"default namespace", "default", model.StateCRDList, true, "default"},
		{"custom namespace", "kube-system", model.StateCRDList, true, "kube-system"},
		{"empty namespace", "", model.StateCRDList, true, ""},
		{"namespace with dash", "my-namespace", model.StateCRDList, true, "my-namespace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &k8s.Client{}
			got := NewModel(mockClient, tt.namespace)
			verifyInitialModel(t, &got, mockClient, expectedModel{tt.wantState, tt.wantLoading, tt.wantNS})
		})
	}
}

type expectedModel struct {
	state   model.ViewState
	loading bool
	ns      string
}

func verifyInitialModel(t *testing.T, got *Model, mockClient *k8s.Client, want expectedModel) {
	t.Helper()

	assert.Equal(t, want.state, got.State)
	assert.Equal(t, want.loading, got.Loading)
	assert.Equal(t, want.ns, got.CurrentNamespace)
	assert.False(t, got.AllNamespaces)
	assert.Equal(t, mockClient, got.K8s)
}
