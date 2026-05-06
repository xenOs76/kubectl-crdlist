package model //nolint:revive

// CRDInfo holds basic info about a CRD.
type CRDInfo struct {
	Name       string
	Group      string
	Version    string
	Resource   string
	Namespaced bool
}

// ResourceInfo holds info about a specific resource instance.
type ResourceInfo struct {
	Name      string
	Namespace string
	CRD       CRDInfo
}

// ViewState represents the current view in the TUI.
type ViewState int

const (
	StateCRDList ViewState = iota
	StateResourceList
	StateGroupResourceList
	StateYAMLView
)

// InputMode represents the current input mode (browsing or filtering).
type InputMode int

const (
	ModeBrowsing InputMode = iota
	ModeFiltering
)

// Messages used for async data fetching
type (
	CRDsLoadedMsg      []CRDInfo
	ResourcesLoadedMsg []ResourceInfo
	YAMLLoadedMsg      string
	ErrMsg             error
)
