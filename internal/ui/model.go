package ui

import (
	"context"

	"github.com/xenos76/kubectl-crdlist/internal/k8s"
	"github.com/xenos76/kubectl-crdlist/internal/model"
)

// Model represents the state of the TUI application.
type Model struct {
	Ctx    context.Context
	Cancel context.CancelFunc

	State  model.ViewState
	Mode   model.InputMode
	K8s    *k8s.Client
	Width  int
	Height int

	Err     error
	Loading bool
	Msg     string

	// Common filter field used by applyFilter and UI rendering
	Filter string

	// Per-view filter storage
	CRDFilter           string
	ResourceFilter      string
	GroupResourceFilter string

	// CRD List state
	Crds            []model.CRDInfo
	FilteredCRDs    []model.CRDInfo
	CrdCursor       int
	CrdScrollOffset int

	// Resource List state
	Resources         []model.ResourceInfo
	FilteredResources []model.ResourceInfo
	ResourceCursor    int
	ResScrollOffset   int
	AllNamespaces     bool
	CurrentNamespace  string // namespace from context/flags
	SelectedCRD       model.CRDInfo
	SelectedGroup     string

	// YAML View state
	SelectedYAML   string
	YamlScrollLine int
	SelectedRes    model.ResourceInfo
}

// NewModel creates a new TUI model with the provided dependencies.
func NewModel(ctx context.Context, cancel context.CancelFunc, k *k8s.Client, ns string) Model {
	return Model{
		Ctx:              ctx,
		Cancel:           cancel,
		State:            model.StateCRDList,
		K8s:              k,
		Loading:          true,
		CurrentNamespace: ns,
		AllNamespaces:    false,
	}
}
