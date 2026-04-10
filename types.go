package main

type viewState int

const (
	stateCRDList viewState = iota
	stateResourceList
	stateGroupResourceList
	stateYAMLView
)

type inputMode int

const (
	modeBrowsing inputMode = iota
	modeFiltering
)

type model struct {
	state  viewState
	mode   inputMode
	k8s    *k8sClient
	width  int
	height int

	err     error
	loading bool
	msg     string

	// CRD List state
	crds            []crdInfo
	filteredCRDs    []crdInfo
	crdCursor       int
	crdScrollOffset int
	filter          string

	// Resource List state
	resources         []resourceInfo
	filteredResources []resourceInfo
	resourceCursor    int
	resScrollOffset   int
	allNamespaces     bool
	currentNamespace  string // namespace from context/flags
	selectedCRD       crdInfo
	selectedGroup     string

	// YAML View state
	selectedYAML   string
	yamlScrollLine int
	selectedRes    resourceInfo
}

func initialModel(k *k8sClient, ns string) model {
	return model{
		state:            stateCRDList,
		k8s:              k,
		loading:          true,
		currentNamespace: ns,
		allNamespaces:    false,
	}
}

// Commands used for async data fetching
type (
	crdsLoadedMsg      []crdInfo
	resourcesLoadedMsg []resourceInfo
	yamlLoadedMsg      string
	errMsg             error
)
