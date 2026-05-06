package ui

import (
	"github.com/xenos76/kubectl-crdlist/internal/k8s"
	"github.com/xenos76/kubectl-crdlist/internal/model"
)

type Model struct {
	State  model.ViewState
	Mode   model.InputMode
	K8s    *k8s.Client
	Width  int
	Height int

	Err     error
	Loading bool
	Msg     string

	// CRD List state
	Crds            []model.CRDInfo
	FilteredCRDs    []model.CRDInfo
	CrdCursor       int
	CrdScrollOffset int
	Filter          string

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

func NewModel(k *k8s.Client, ns string) Model {
	return Model{
		State:            model.StateCRDList,
		K8s:              k,
		Loading:          true,
		CurrentNamespace: ns,
		AllNamespaces:    false,
	}
}
