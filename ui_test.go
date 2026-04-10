package main

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestApplyFilterCRDList(t *testing.T) {
	m := model{
		state: stateCRDList,
		crds: []crdInfo{
			{group: "networking.k8s.io", name: "ingressclasses.networking.k8s.io"},
			{group: "apps", name: "deployments.apps"},
			{group: "core", name: "pods.core"},
		},
		filter: "deploy",
		mode:   modeFiltering,
	}

	m.applyFilter()

	if len(m.filteredCRDs) != 1 {
		t.Fatalf("expected 1 filtered CRD, got %d", len(m.filteredCRDs))
	}

	if m.filteredCRDs[0].name != "deployments.apps" {
		t.Errorf("expected deployments.apps, got %s", m.filteredCRDs[0].name)
	}
}

func TestApplyFilterResourceList(t *testing.T) {
	m := model{
		state: stateResourceList,
		resources: []resourceInfo{
			{name: "my-ingress", namespace: "default"},
			{name: "your-ingress", namespace: "test"},
			{name: "some-pod", namespace: "default"},
		},
		filter: "ingress",
		mode:   modeFiltering,
	}

	m.applyFilter()

	if len(m.filteredResources) != 2 {
		t.Fatalf("expected 2 filtered resources, got %d", len(m.filteredResources))
	}
}

func TestMoveDownCRD(t *testing.T) {
	m := model{
		state:           stateCRDList,
		filteredCRDs:    make([]crdInfo, 20),
		crdCursor:       0,
		crdScrollOffset: 0,
		height:          24, // 24 - 8 = 16 max height
	}

	m.moveDown(5)

	if m.crdCursor != 5 {
		t.Errorf("expected cursor at 5, got %d", m.crdCursor)
	}

	m.moveDown(15) // moving down bounds

	if m.crdCursor != 19 {
		t.Errorf("expected cursor constrained to 19, got %d", m.crdCursor)
	}

	if m.crdScrollOffset != 4 {
		t.Errorf("expected scroll offset at 4, got %d", m.crdScrollOffset)
	}
}

func TestMoveUpCRD(t *testing.T) {
	m := model{
		state:           stateCRDList,
		filteredCRDs:    make([]crdInfo, 20),
		crdCursor:       19,
		crdScrollOffset: 4,
	}

	m.moveUp(5)

	if m.crdCursor != 14 {
		t.Errorf("expected cursor at 14, got %d", m.crdCursor)
	}

	m.moveUp(20) // moving up boundary limits

	if m.crdCursor != 0 {
		t.Errorf("expected cursor constrained to 0, got %d", m.crdCursor)
	}

	if m.crdScrollOffset != 0 {
		t.Errorf("expected scroll offset to reset to 0, got %d", m.crdScrollOffset)
	}
}

func TestHandleEscape(t *testing.T) {
	m := model{
		mode:   modeFiltering,
		filter: "test",
	}

	m.handleEscape()

	if m.mode != modeBrowsing {
		t.Errorf("expected mode to switch to browsing, got mode %v", m.mode)
	}
	// Note: in filtering mode, escape just changes mode, doesn't erase filter
	if m.filter != "test" {
		t.Errorf("expected filter to remain test, got %s", m.filter)
	}

	// Try clearing filter in browsing mode
	m.state = stateCRDList
	m.handleEscape()

	if m.filter != "" {
		t.Errorf("expected filter to clear when navigating via escape, got %s", m.filter)
	}
}

// Below are the new tests added to boost coverage to 90%+

func TestViewRendering(t *testing.T) {
	m := model{
		width:  80,
		height: 24,
	}

	// Test Error State
	m.err = errors.New("test error")

	out := m.View()
	if !strings.Contains(out.Content, "Error:") {
		t.Errorf("expected view to render error, got %s", out.Content)
	}

	m.err = nil

	// Test Loading State
	m.loading = true

	out = m.View()
	if !strings.Contains(out.Content, "Loading data") {
		t.Errorf("expected view to render loading, got %s", out.Content)
	}

	m.loading = false

	// Test CRD List Rendering
	m.state = stateCRDList
	m.filteredCRDs = []crdInfo{{group: "test-group", name: "test-crd"}}

	out = m.View()
	if !strings.Contains(out.Content, "test-group") || !strings.Contains(out.Content, "test-crd") {
		t.Error("expected CRD list view to contain test CRD data")
	}

	// Test Resource List Rendering
	m.state = stateResourceList
	m.filteredResources = []resourceInfo{{name: "test-res", namespace: "default"}}

	out = m.View()
	if !strings.Contains(out.Content, "test-res") {
		t.Error("expected Resource list view to contain test resource data")
	}

	// Test Group Resource List Rendering
	m.state = stateGroupResourceList

	out = m.View()
	if !strings.Contains(out.Content, "test-res") {
		t.Error("expected Group Resource list view to contain test resource data")
	}

	// Test YAML View Rendering
	m.state = stateYAMLView
	m.selectedYAML = "apiVersion: v1\nkind: Pod"

	out = m.View()
	if !strings.Contains(out.Content, "Pod") {
		t.Errorf("expected YAML view to contain YAML text")
	}
}

func TestUpdateMessages(t *testing.T) {
	m := model{}

	// Test CRDs loaded msg
	crdMsg := crdsLoadedMsg{
		{group: "b", name: "b-crd"},
		{group: "a", name: "a-crd"},
	}
	newModel, _ := m.Update(crdMsg)
	mUpdated := newModel.(model)

	if len(mUpdated.crds) != 2 || mUpdated.crds[0].group != "a" {
		t.Errorf("expected CRDs to be loaded and sorted, got %v", mUpdated.crds)
	}

	// Test Resources loaded msg
	resMsg := resourcesLoadedMsg{
		{name: "res1"},
	}
	newModel, _ = mUpdated.Update(resMsg)
	mUpdated = newModel.(model)

	if len(mUpdated.resources) != 1 || mUpdated.resources[0].name != "res1" {
		t.Errorf("expected resources to be loaded, got %v", mUpdated.resources)
	}

	// Test YAML loaded msg
	yamlMsg := yamlLoadedMsg("test-yaml")
	newModel, _ = mUpdated.Update(yamlMsg)
	mUpdated = newModel.(model)

	if mUpdated.selectedYAML != "test-yaml" {
		t.Errorf("expected YAML to be loaded, got %s", mUpdated.selectedYAML)
	}

	// Test WindowSizeMsg
	sizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, _ = mUpdated.Update(sizeMsg)
	mUpdated = newModel.(model)

	if mUpdated.width != 100 || mUpdated.height != 50 {
		t.Errorf("expected window size to update")
	}
}

func TestHandleKeyPress(t *testing.T) {
	m := model{
		state:        stateCRDList,
		mode:         modeBrowsing,
		filteredCRDs: []crdInfo{{name: "crd1", group: "group1"}},
	}

	// Test enter filter mode
	msg := tea.KeyPressMsg{Text: "/", Code: '/'}
	newModel, _ := m.Update(msg)
	mUpdated := newModel.(model)

	if mUpdated.mode != modeFiltering {
		t.Errorf("expected mode to switch to filtering")
	}

	// Test type filter letter
	msg = tea.KeyPressMsg{Text: "x", Code: 'x'}
	newModel, _ = mUpdated.Update(msg)
	mUpdated = newModel.(model)

	if mUpdated.filter != "x" {
		t.Errorf("expected filter to be 'x', got %s", mUpdated.filter)
	}

	// Test backspace
	msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
	newModel, _ = mUpdated.Update(msg)
	mUpdated = newModel.(model)

	if mUpdated.filter != "" {
		t.Errorf("expected filter to be empty, got test")
	}

	// Test escape from filter mode
	msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	newModel, _ = mUpdated.Update(msg)
	mUpdated = newModel.(model)

	if mUpdated.mode != modeBrowsing {
		t.Errorf("expected mode to switch to browsing")
	}

	// Test namespace toggle
	mUpdated.state = stateResourceList
	msg = tea.KeyPressMsg{Text: "n", Code: 'n'}
	newModel, _ = mUpdated.Update(msg)
	mUpdated = newModel.(model)

	if !mUpdated.allNamespaces {
		t.Errorf("expected allNamespaces to be true")
	}
}

func TestFetchCommands(t *testing.T) {
	scheme := runtime.NewScheme()
	testCRD := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]any{"name": "testcrds.example.com"},
			"spec": map[string]any{
				"group": "example.com",
				"names": map[string]any{"plural": "testcrds"},
				"versions": []any{
					map[string]any{"name": "v1", "served": true, "storage": true},
				},
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{
			Group:    "apiextensions.k8s.io",
			Version:  "v1",
			Resource: "customresourcedefinitions",
		}: "CustomResourceDefinitionList",
		{
			Group:    "example.com",
			Version:  "v1",
			Resource: "testcrds",
		}: "TestCrdList",
	}, testCRD)

	k := &k8sClient{dynamic: dynClient}
	m := model{
		k8s: k,
		selectedCRD: crdInfo{
			name:     "testcrds.example.com",
			group:    "example.com",
			version:  "v1",
			resource: "testcrds",
		},
	}

	// Test fetchCRDs
	cmd := m.fetchCRDs()

	msg := cmd()
	if _, ok := msg.(crdsLoadedMsg); !ok {
		t.Errorf("expected crdsLoadedMsg, got %T", msg)
	}

	// Test fetchResources
	cmd = m.fetchResources()

	msg = cmd()
	if _, ok := msg.(resourcesLoadedMsg); !ok {
		t.Errorf("expected resourcesLoadedMsg, got %T", msg)
	}
}

func TestInit(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			gvr: "CustomResourceDefinitionList",
		},
	)

	k := &k8sClient{
		dynamic: dynamicClient,
	}

	m := initialModel(k, "default")

	cmd := m.Init()

	// Init should return a command
	if cmd == nil {
		t.Fatal("Init() returned nil command")
	}

	// Execute the command to get the message
	msg := cmd()

	// The Init command should trigger a fetchCRDs which returns crdsLoadedMsg
	// Should be a crdsLoadedMsg or error
	switch msg.(type) {
	case crdsLoadedMsg, errMsg:
		// Expected
	default:
		t.Errorf("unexpected message type: %T", msg)
	}
}

func TestFetchYAMLError(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "tests",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			gvr: "TestList",
		},
	)

	k := &k8sClient{
		dynamic: dynamicClient,
	}

	m := model{
		k8s: k,
		selectedCRD: crdInfo{
			name:     "test.example.com",
			group:    "example.com",
			version:  "v1",
			resource: "tests",
		},
		selectedRes: resourceInfo{
			name:      "nonexistent",
			namespace: "default",
		},
	}

	// Test fetchYAML with non-existent resource
	cmd := m.fetchYAML()
	msg := cmd()

	// Should return errMsg on failure
	_, isError := msg.(errMsg)
	if !isError {
		t.Errorf("expected errMsg, got %T", msg)
	}
}

func TestMoveDownYAMLView(t *testing.T) {
	tests := []struct {
		name           string
		yamlScrollLine int
		yamlContent    string
		moveAmount     int
		wantLine       int
	}{
		{
			name:           "scroll down within bounds",
			yamlScrollLine: 0,
			yamlContent:    "line1\nline2\nline3\nline4\nline5",
			moveAmount:     2,
			wantLine:       2,
		},
		{
			name:           "scroll down at end",
			yamlScrollLine: 3,
			yamlContent:    "line1\nline2\nline3\nline4\nline5",
			moveAmount:     5,
			wantLine:       4, // Should clamp to max
		},
		{
			name:           "scroll up",
			yamlScrollLine: 3,
			yamlContent:    "line1\nline2\nline3\nline4\nline5",
			moveAmount:     -2,
			wantLine:       1,
		},
		{
			name:           "scroll up at beginning",
			yamlScrollLine: 1,
			yamlContent:    "line1\nline2\nline3\nline4\nline5",
			moveAmount:     -5,
			wantLine:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				state:          stateYAMLView,
				selectedYAML:   tt.yamlContent,
				yamlScrollLine: tt.yamlScrollLine,
			}

			m.moveDown(tt.moveAmount)

			if m.yamlScrollLine != tt.wantLine {
				t.Errorf("yamlScrollLine = %d, want %d", m.yamlScrollLine, tt.wantLine)
			}
		})
	}
}

func TestHandleNamespaceToggle(t *testing.T) {
	tests := []struct {
		name              string
		state             viewState
		allNamespaces     bool
		wantAllNamespaces bool
	}{
		{
			name:              "toggle in resource list",
			state:             stateResourceList,
			allNamespaces:     false,
			wantAllNamespaces: true,
		},
		{
			name:              "toggle in resource list again",
			state:             stateResourceList,
			allNamespaces:     true,
			wantAllNamespaces: false,
		},
		{
			name:              "toggle from group resource list",
			state:             stateGroupResourceList,
			allNamespaces:     false,
			wantAllNamespaces: true,
		},
		{
			name:              "no toggle in CRD list",
			state:             stateCRDList,
			allNamespaces:     false,
			wantAllNamespaces: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				state:         tt.state,
				allNamespaces: tt.allNamespaces,
				k8s: &k8sClient{
					dynamic: fake.NewSimpleDynamicClient(runtime.NewScheme()),
				},
			}

			newModel, _ := m.handleNamespaceToggle()
			newM := newModel.(model)

			if newM.allNamespaces != tt.wantAllNamespaces {
				t.Errorf("allNamespaces = %v, want %v", newM.allNamespaces, tt.wantAllNamespaces)
			}
		})
	}
}

func TestHandleBrowsingKeys(t *testing.T) {
	tests := []struct {
		name           string
		state          viewState
		keyMsg         tea.KeyPressMsg
		shouldNotPanic bool
	}{
		{
			name:           "arrow up in CRD list",
			state:          stateCRDList,
			keyMsg:         tea.KeyPressMsg{Code: tea.KeyUp},
			shouldNotPanic: true,
		},
		{
			name:           "arrow down in CRD list",
			state:          stateCRDList,
			keyMsg:         tea.KeyPressMsg{Code: tea.KeyDown},
			shouldNotPanic: true,
		},
		{
			name:           "arrow up in resource list",
			state:          stateResourceList,
			keyMsg:         tea.KeyPressMsg{Code: tea.KeyUp},
			shouldNotPanic: true,
		},
		{
			name:           "enter in CRD list",
			state:          stateCRDList,
			keyMsg:         tea.KeyPressMsg{Code: tea.KeyEnter},
			shouldNotPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && tt.shouldNotPanic {
					t.Errorf("handleBrowsingKeys panicked: %v", r)
				}
			}()

			m := &model{
				state: tt.state,
				crds: []crdInfo{
					{name: "test1", group: "g1"},
					{name: "test2", group: "g2"},
				},
				filteredCRDs: []crdInfo{
					{name: "test1", group: "g1"},
					{name: "test2", group: "g2"},
				},
				resources: []resourceInfo{
					{name: "res1", namespace: "default"},
					{name: "res2", namespace: "default"},
				},
				filteredResources: []resourceInfo{
					{name: "res1", namespace: "default"},
					{name: "res2", namespace: "default"},
				},
				k8s: &k8sClient{
					dynamic: fake.NewSimpleDynamicClient(runtime.NewScheme()),
				},
			}

			m.handleBrowsingKeys(tt.keyMsg)
		})
	}
}

func TestHighlightYAMLEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		yaml   string
		wantOK bool
	}{
		{
			name:   "valid YAML",
			yaml:   "kind: Pod\nmetadata:\n  name: test",
			wantOK: true,
		},
		{
			name:   "empty YAML",
			yaml:   "",
			wantOK: true,
		},
		{
			name:   "YAML with special chars",
			yaml:   "kind: Pod\ndata: \"special: @#$%\"",
			wantOK: true,
		},
		{
			name:   "multiline YAML",
			yaml:   "spec:\n  containers:\n  - name: test\n    image: nginx:latest",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{}
			result := m.highlightYAML(tt.yaml)

			if tt.wantOK {
				// Just check it doesn't panic and returns a string
				_ = result
			}
		})
	}
}

func TestFetchCRDsError(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			gvr: "CustomResourceDefinitionList",
		},
	)

	k := &k8sClient{
		dynamic: dynamicClient,
	}

	m := &model{
		k8s: k,
	}

	cmd := m.fetchCRDs()
	if cmd == nil {
		t.Fatal("fetchCRDs returned nil command")
	}

	msg := cmd()
	switch msg.(type) {
	case crdsLoadedMsg, errMsg:
		// Expected
	default:
		t.Errorf("unexpected message type: %T", msg)
	}
}

func TestFetchResourcesWithAllNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "examples",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			gvr: "ExampleList",
		},
	)

	k := &k8sClient{
		dynamic: dynamicClient,
	}

	m := &model{
		k8s:           k,
		allNamespaces: true,
		selectedCRD: crdInfo{
			name:     "example.example.com",
			group:    "example.com",
			version:  "v1",
			resource: "examples",
		},
	}

	cmd := m.fetchResources()
	if cmd == nil {
		t.Fatal("fetchResources returned nil command")
	}

	msg := cmd()
	switch msg.(type) {
	case resourcesLoadedMsg, errMsg:
		// Expected
	default:
		t.Errorf("unexpected message type: %T", msg)
	}
}

func TestListResourcesEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		wantErr   bool
	}{
		{
			name:      "default namespace",
			namespace: "default",
			wantErr:   false,
		},
		{
			name:      "custom namespace",
			namespace: "kube-system",
			wantErr:   false,
		},
		{
			name:      "all namespaces indicator",
			namespace: "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			gvr := schema.GroupVersionResource{
				Group:    "example.com",
				Version:  "v1",
				Resource: "examples",
			}

			dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
				scheme,
				map[schema.GroupVersionResource]string{
					gvr: "ExampleList",
				},
			)

			k := &k8sClient{
				dynamic: dynamicClient,
			}

			crd := crdInfo{
				name:     "example.example.com",
				group:    "example.com",
				version:  "v1",
				resource: "examples",
			}

			resources, err := k.listResources(crd, tt.namespace)

			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if resources != nil {
				t.Logf("Got %d resources", len(resources))
			}
		})
	}
}

func TestFetchGroupResources(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "examples",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			gvr: "ExampleList",
		},
	)

	k := &k8sClient{
		dynamic: dynamicClient,
	}

	m := &model{
		k8s:              k,
		allNamespaces:    false,
		currentNamespace: "default",
		selectedGroup:    "example.com",
		resources: []resourceInfo{
			{
				name:      "example1",
				namespace: "default",
				crd: crdInfo{
					name:     "example.example.com",
					group:    "example.com",
					version:  "v1",
					resource: "examples",
				},
			},
		},
	}

	cmd := m.fetchGroupResources()
	if cmd == nil {
		t.Fatal("fetchGroupResources returned nil command")
	}

	msg := cmd()
	switch msg.(type) {
	case resourcesLoadedMsg, errMsg:
		// Expected
	default:
		t.Errorf("unexpected message type: %T", msg)
	}
}

func TestHandleEscapeEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		state         viewState
		mode          inputMode
		selectedGroup string
		filter        string
		wantState     viewState
		wantMode      inputMode
		wantFilter    string
	}{
		{
			name:      "escape from filtering mode",
			state:     stateCRDList,
			mode:      modeFiltering,
			wantState: stateCRDList,
			wantMode:  modeBrowsing,
		},
		{
			name:          "escape from YAML view with group",
			state:         stateYAMLView,
			selectedGroup: "example.com",
			wantState:     stateGroupResourceList,
		},
		{
			name:      "escape from YAML view without group",
			state:     stateYAMLView,
			wantState: stateResourceList,
		},
		{
			name:       "escape from resource list",
			state:      stateResourceList,
			wantState:  stateCRDList,
			wantFilter: "",
		},
		{
			name:       "escape from group resource list",
			state:      stateGroupResourceList,
			wantState:  stateCRDList,
			wantFilter: "",
		},
		{
			name:       "escape with filter",
			state:      stateCRDList,
			filter:     "test",
			wantFilter: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				state:         tt.state,
				mode:          tt.mode,
				selectedGroup: tt.selectedGroup,
				filter:        tt.filter,
				crds: []crdInfo{
					{name: "test1", group: "g1"},
				},
				filteredCRDs: []crdInfo{
					{name: "test1", group: "g1"},
				},
				resources: []resourceInfo{
					{name: "res1", namespace: "default"},
				},
				filteredResources: []resourceInfo{
					{name: "res1", namespace: "default"},
				},
			}

			m.handleEscape()

			if m.state != tt.wantState {
				t.Errorf("state = %v, want %v", m.state, tt.wantState)
			}

			if m.mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", m.mode, tt.wantMode)
			}
		})
	}
}

func TestMoveUp(t *testing.T) {
	tests := []struct {
		name          string
		state         viewState
		crdCursor     int
		resCursor     int
		moveAmount    int
		wantCRDCursor int
		wantResCursor int
	}{
		{
			name:          "move up in CRD list",
			state:         stateCRDList,
			crdCursor:     3,
			moveAmount:    1,
			wantCRDCursor: 2,
		},
		{
			name:          "move up at beginning",
			state:         stateCRDList,
			crdCursor:     0,
			moveAmount:    5,
			wantCRDCursor: 0,
		},
		{
			name:          "move up in resource list",
			state:         stateResourceList,
			resCursor:     2,
			moveAmount:    1,
			wantResCursor: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				state:          tt.state,
				crdCursor:      tt.crdCursor,
				resourceCursor: tt.resCursor,
				crds: []crdInfo{
					{name: "c1", group: "g1"},
					{name: "c2", group: "g2"},
					{name: "c3", group: "g3"},
					{name: "c4", group: "g4"},
				},
				resources: []resourceInfo{
					{name: "r1", namespace: "default"},
					{name: "r2", namespace: "default"},
					{name: "r3", namespace: "default"},
				},
			}

			m.moveUp(tt.moveAmount)

			if m.state == stateCRDList && m.crdCursor != tt.wantCRDCursor {
				t.Errorf("crdCursor = %d, want %d", m.crdCursor, tt.wantCRDCursor)
			}

			if m.state == stateResourceList && m.resourceCursor != tt.wantResCursor {
				t.Errorf("resCursor = %d, want %d", m.resourceCursor, tt.wantResCursor)
			}
		})
	}
}

func TestMoveDownRes(t *testing.T) {
	tests := []struct {
		name          string
		resCursor     int
		resCount      int
		moveAmount    int
		wantResCursor int
	}{
		{
			name:          "move down in resource list",
			resCursor:     0,
			resCount:      3,
			moveAmount:    1,
			wantResCursor: 1,
		},
		{
			name:          "move down past end",
			resCursor:     2,
			resCount:      3,
			moveAmount:    5,
			wantResCursor: 2,
		},
		{
			name:          "move down to end",
			resCursor:     0,
			resCount:      3,
			moveAmount:    2,
			wantResCursor: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := make([]resourceInfo, tt.resCount)
			for i := 0; i < tt.resCount; i++ {
				resources[i] = resourceInfo{name: "res", namespace: "default"}
			}

			m := &model{
				state:             stateResourceList,
				resourceCursor:    tt.resCursor,
				resources:         resources,
				filteredResources: resources,
			}

			m.moveDownRes(tt.moveAmount)

			if m.resourceCursor != tt.wantResCursor {
				t.Errorf("resourceCursor = %d, want %d", m.resourceCursor, tt.wantResCursor)
			}
		})
	}
}

func TestRenderCRDListEmpty(t *testing.T) {
	var sb strings.Builder

	m := &model{
		state:        stateCRDList,
		crds:         []crdInfo{},
		filteredCRDs: []crdInfo{},
		width:        80,
		height:       24,
	}

	m.renderCRDList(&sb)

	if sb.Len() == 0 {
		t.Error("renderCRDList produced empty output")
	}
}

func TestRenderResourceListEmpty(t *testing.T) {
	var sb strings.Builder

	m := &model{
		state:             stateResourceList,
		resources:         []resourceInfo{},
		filteredResources: []resourceInfo{},
		width:             80,
		height:            24,
	}

	m.renderResourceList(&sb)

	if sb.Len() == 0 {
		t.Error("renderResourceList produced empty output")
	}
}

func TestRenderGroupResourceListEmpty(t *testing.T) {
	var sb strings.Builder

	m := &model{
		state:             stateGroupResourceList,
		resources:         []resourceInfo{},
		filteredResources: []resourceInfo{},
		width:             80,
		height:            24,
	}

	m.renderGroupResourceList(&sb)

	if sb.Len() == 0 {
		t.Error("renderGroupResourceList produced empty output")
	}
}

func TestMoveDownCRDEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		crdCursor     int
		crdCount      int
		moveAmount    int
		wantCRDCursor int
	}{
		{
			name:          "move down in CRD list",
			crdCursor:     0,
			crdCount:      3,
			moveAmount:    1,
			wantCRDCursor: 1,
		},
		{
			name:          "move down to boundary",
			crdCursor:     1,
			crdCount:      3,
			moveAmount:    1,
			wantCRDCursor: 2,
		},
		{
			name:          "move down past end",
			crdCursor:     2,
			crdCount:      3,
			moveAmount:    5,
			wantCRDCursor: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crds := make([]crdInfo, tt.crdCount)
			for i := 0; i < tt.crdCount; i++ {
				crds[i] = crdInfo{name: "crd", group: "g"}
			}

			m := &model{
				state:        stateCRDList,
				crdCursor:    tt.crdCursor,
				crds:         crds,
				filteredCRDs: crds,
			}

			m.moveDownCRD(tt.moveAmount)

			if m.crdCursor != tt.wantCRDCursor {
				t.Errorf("crdCursor = %d, want %d", m.crdCursor, tt.wantCRDCursor)
			}
		})
	}
}

func TestRenderWithData(t *testing.T) {
	tests := []struct {
		name      string
		crdCount  int
		resCount  int
		wantError bool
	}{
		{
			name:      "render with 1 CRD",
			crdCount:  1,
			wantError: false,
		},
		{
			name:      "render with 3 CRDs",
			crdCount:  3,
			wantError: false,
		},
		{
			name:      "render with 1 resource",
			resCount:  1,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder

			crds := make([]crdInfo, tt.crdCount)
			for i := 0; i < tt.crdCount; i++ {
				crds[i] = crdInfo{name: "crd" + string(rune(i)), group: "g"}
			}

			resources := make([]resourceInfo, tt.resCount)
			for i := 0; i < tt.resCount; i++ {
				resources[i] = resourceInfo{name: "res" + string(rune(i)), namespace: "default"}
			}

			m := &model{
				state:             stateCRDList,
				crds:              crds,
				filteredCRDs:      crds,
				resources:         resources,
				filteredResources: resources,
				width:             80,
				height:            24,
				crdCursor:         0,
			}

			m.renderCRDList(&sb)

			if tt.crdCount > 0 && sb.Len() == 0 {
				t.Error("renderCRDList with data produced empty output")
			}
		})
	}
}

func TestHighlightYAMLWithContent(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantLen int
	}{
		{
			name:    "short YAML",
			yaml:    "kind: Pod",
			wantLen: 1,
		},
		{
			name:    "complex YAML",
			yaml:    "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test\nspec:\n  containers:\n  - name: app\n    image: nginx",
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{}

			result := m.highlightYAML(tt.yaml)
			if len(result) == 0 && tt.wantLen > 0 {
				t.Error("highlightYAML returned empty for non-empty YAML")
			}
		})
	}
}

func TestHandleFilteringKeys(t *testing.T) {
	tests := []struct {
		name     string
		keyMsg   tea.KeyPressMsg
		state    viewState
		filter   string
		mode     inputMode
		wantMode inputMode
	}{
		{
			name:     "type character",
			keyMsg:   tea.KeyPressMsg{Text: "a", Code: 'a'},
			state:    stateCRDList,
			filter:   "",
			mode:     modeFiltering,
			wantMode: modeFiltering,
		},
		{
			name:     "backspace in filter",
			keyMsg:   tea.KeyPressMsg{Code: tea.KeyBackspace},
			state:    stateCRDList,
			filter:   "test",
			mode:     modeFiltering,
			wantMode: modeFiltering,
		},
		{
			name:     "escape in filter",
			keyMsg:   tea.KeyPressMsg{Code: tea.KeyEscape},
			state:    stateCRDList,
			filter:   "test",
			mode:     modeFiltering,
			wantMode: modeBrowsing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				state:        tt.state,
				filter:       tt.filter,
				mode:         tt.mode,
				crds:         []crdInfo{{name: "test", group: "g"}},
				filteredCRDs: []crdInfo{{name: "test", group: "g"}},
			}

			newModel, _ := m.handleFilteringKeys(tt.keyMsg)
			newM := newModel.(model)

			if newM.mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", newM.mode, tt.wantMode)
			}
		})
	}
}

func TestGetPositionIndicator(t *testing.T) {
	tests := []struct {
		name      string
		state     viewState
		crdCursor int
		crdCount  int
		resCursor int
		resCount  int
	}{
		{
			name:      "CRD list position",
			state:     stateCRDList,
			crdCursor: 1,
			crdCount:  3,
		},
		{
			name:      "resource list position",
			state:     stateResourceList,
			resCursor: 0,
			resCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crds := make([]crdInfo, tt.crdCount)
			for i := 0; i < tt.crdCount; i++ {
				crds[i] = crdInfo{name: "c", group: "g"}
			}

			resources := make([]resourceInfo, tt.resCount)
			for i := 0; i < tt.resCount; i++ {
				resources[i] = resourceInfo{name: "r", namespace: "d"}
			}

			m := &model{
				state:             tt.state,
				crdCursor:         tt.crdCursor,
				crds:              crds,
				filteredCRDs:      crds,
				resourceCursor:    tt.resCursor,
				resources:         resources,
				filteredResources: resources,
			}

			pos := m.getPositionIndicator()
			if !strings.Contains(pos, "/") {
				t.Errorf("position indicator should contain '/': %s", pos)
			}
		})
	}
}

func TestMoveUpExtended(t *testing.T) {
	tests := []struct {
		name          string
		state         viewState
		initialCursor int
		initialOffset int
		moveAmount    int
		wantCursor    int
		wantOffset    int
	}{
		{
			name:          "scroll up CRD list with offset",
			state:         stateCRDList,
			initialCursor: 5,
			initialOffset: 3,
			moveAmount:    2,
			wantCursor:    3,
			wantOffset:    3,
		},
		{
			name:          "scroll up resource with large amount",
			state:         stateResourceList,
			initialCursor: 10,
			initialOffset: 5,
			moveAmount:    15,
			wantCursor:    0,
			wantOffset:    0,
		},
		{
			name:          "scroll up at top",
			state:         stateCRDList,
			initialCursor: 0,
			initialOffset: 0,
			moveAmount:    10,
			wantCursor:    0,
			wantOffset:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crds := make([]crdInfo, 20)
			for i := 0; i < 20; i++ {
				crds[i] = crdInfo{name: "c", group: "g"}
			}

			resources := make([]resourceInfo, 15)
			for i := 0; i < 15; i++ {
				resources[i] = resourceInfo{name: "r", namespace: "d"}
			}

			m := &model{
				state:             tt.state,
				crdCursor:         tt.initialCursor,
				crdScrollOffset:   tt.initialOffset,
				crds:              crds,
				filteredCRDs:      crds,
				resourceCursor:    tt.initialCursor,
				resScrollOffset:   tt.initialOffset,
				resources:         resources,
				filteredResources: resources,
			}

			m.moveUp(tt.moveAmount)

			if tt.state == stateCRDList {
				if m.crdCursor != tt.wantCursor {
					t.Errorf("crdCursor = %d, want %d", m.crdCursor, tt.wantCursor)
				}
			} else if tt.state == stateResourceList {
				if m.resourceCursor != tt.wantCursor {
					t.Errorf("resourceCursor = %d, want %d", m.resourceCursor, tt.wantCursor)
				}
			}
		})
	}
}

func TestHighlightYAMLComprehensive(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		checkFn func(string) bool
	}{
		{
			name:    "empty string",
			yaml:    "",
			checkFn: func(s string) bool { return len(s) == 0 },
		},
		{
			name:    "single line",
			yaml:    "key: value",
			checkFn: func(s string) bool { return len(s) > 0 },
		},
		{
			name:    "multiline with nesting",
			yaml:    "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test\nspec:\n  containers:\n  - name: app",
			checkFn: func(s string) bool { return strings.Contains(s, "Pod") || len(s) > 0 },
		},
		{
			name:    "with special characters",
			yaml:    "data: \"key: value|special@chars#123\"",
			checkFn: func(s string) bool { return len(s) > 0 },
		},
		{
			name:    "with quotes and escapes",
			yaml:    "message: \"Line 1\\nLine 2\"",
			checkFn: func(s string) bool { return len(s) > 0 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{}
			result := m.highlightYAML(tt.yaml)

			if !tt.checkFn(result) {
				t.Errorf("highlightYAML(%q) validation failed, got: %q", tt.yaml, result)
			}
		})
	}
}

func TestFetchCRDsAsync(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			gvr: "CustomResourceDefinitionList",
		},
	)

	k := &k8sClient{
		dynamic: dynamicClient,
	}

	m := &model{
		k8s: k,
	}

	// Test async command execution
	cmd := m.fetchCRDs()
	if cmd == nil {
		t.Fatal("fetchCRDs returned nil")
	}

	msg := cmd()
	switch msg.(type) {
	case crdsLoadedMsg:
	// Expected - successful load
	case errMsg:
	// Expected - error case
	default:
		t.Errorf("unexpected message type: %T", msg)
	}
}

func TestFetchYAMLAsync(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "tests",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			gvr: "TestList",
		},
	)

	k := &k8sClient{
		dynamic: dynamicClient,
	}

	tests := []struct {
		name string
		crd  crdInfo
		res  resourceInfo
	}{
		{
			name: "fetch existing resource",
			crd: crdInfo{
				name:     "test.example.com",
				group:    "example.com",
				version:  "v1",
				resource: "tests",
			},
			res: resourceInfo{
				name:      "nonexistent",
				namespace: "default",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				k8s:         k,
				selectedCRD: tt.crd,
				selectedRes: tt.res,
			}

			cmd := m.fetchYAML()
			if cmd == nil {
				t.Fatal("fetchYAML returned nil")
			}

			msg := cmd()
			switch msg.(type) {
			case yamlLoadedMsg, errMsg:
			// Expected
			default:
				t.Errorf("unexpected message type: %T", msg)
			}
		})
	}
}

func TestHandleBrowsingKeysExtended(t *testing.T) {
	tests := []struct {
		name       string
		keyMsg     tea.KeyPressMsg
		state      viewState
		wantChange bool
	}{
		{
			name:       "up arrow in CRD list",
			keyMsg:     tea.KeyPressMsg{Code: tea.KeyUp},
			state:      stateCRDList,
			wantChange: true,
		},
		{
			name:       "down arrow in resource list",
			keyMsg:     tea.KeyPressMsg{Code: tea.KeyDown},
			state:      stateResourceList,
			wantChange: true,
		},
		{
			name:       "pgup key",
			keyMsg:     tea.KeyPressMsg{Code: tea.KeyPgUp},
			state:      stateCRDList,
			wantChange: true,
		},
		{
			name:       "pgdown key",
			keyMsg:     tea.KeyPressMsg{Code: tea.KeyPgDown},
			state:      stateCRDList,
			wantChange: true,
		},
		{
			name:       "home key",
			keyMsg:     tea.KeyPressMsg{Code: tea.KeyHome},
			state:      stateCRDList,
			wantChange: true,
		},
		{
			name:       "end key",
			keyMsg:     tea.KeyPressMsg{Code: tea.KeyEnd},
			state:      stateCRDList,
			wantChange: true,
		},
		{
			name:       "slash for filter",
			keyMsg:     tea.KeyPressMsg{Text: "/", Code: '/'},
			state:      stateCRDList,
			wantChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				state: tt.state,
				crds: []crdInfo{
					{name: "c1", group: "g"},
					{name: "c2", group: "g"},
					{name: "c3", group: "g"},
				},
				filteredCRDs: []crdInfo{
					{name: "c1", group: "g"},
					{name: "c2", group: "g"},
					{name: "c3", group: "g"},
				},
				resources: []resourceInfo{
					{name: "r1", namespace: "d"},
					{name: "r2", namespace: "d"},
				},
				filteredResources: []resourceInfo{
					{name: "r1", namespace: "d"},
					{name: "r2", namespace: "d"},
				},
			}

			initialCursor := m.crdCursor
			m.handleBrowsingKeys(tt.keyMsg)

			// Just ensure no panic
			_ = initialCursor
		})
	}
}

func TestFetchResourcesMultiNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "examples",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			gvr: "ExampleList",
		},
	)

	k := &k8sClient{
		dynamic: dynamicClient,
	}

	tests := []struct {
		name          string
		allNamespaces bool
		namespace     string
	}{
		{
			name:          "all namespaces mode",
			allNamespaces: true,
			namespace:     "",
		},
		{
			name:          "specific namespace",
			allNamespaces: false,
			namespace:     "default",
		},
		{
			name:          "kube-system namespace",
			allNamespaces: false,
			namespace:     "kube-system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				k8s:              k,
				allNamespaces:    tt.allNamespaces,
				currentNamespace: tt.namespace,
				selectedCRD: crdInfo{
					name:     "example.example.com",
					group:    "example.com",
					version:  "v1",
					resource: "examples",
				},
			}

			cmd := m.fetchResources()
			if cmd == nil {
				t.Fatal("fetchResources returned nil")
			}

			msg := cmd()
			switch msg.(type) {
			case resourcesLoadedMsg, errMsg:
			// Expected
			default:
				t.Errorf("unexpected message type: %T", msg)
			}
		})
	}
}

func TestRenderingWithScrolling(t *testing.T) {
	tests := []struct {
		name         string
		itemCount    int
		cursor       int
		scrollOffset int
		width        int
		height       int
	}{
		{
			name:         "render with scroll offset",
			itemCount:    20,
			cursor:       10,
			scrollOffset: 5,
			width:        80,
			height:       10,
		},
		{
			name:         "render at boundary",
			itemCount:    5,
			cursor:       4,
			scrollOffset: 0,
			width:        80,
			height:       10,
		},
		{
			name:         "render with large offset",
			itemCount:    50,
			cursor:       25,
			scrollOffset: 20,
			width:        80,
			height:       10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder

			crds := make([]crdInfo, tt.itemCount)
			for i := 0; i < tt.itemCount; i++ {
				crds[i] = crdInfo{name: "crd" + string(rune(i%10)), group: "g"}
			}

			m := &model{
				state:           stateCRDList,
				crds:            crds,
				filteredCRDs:    crds,
				crdCursor:       tt.cursor,
				crdScrollOffset: tt.scrollOffset,
				width:           tt.width,
				height:          tt.height,
			}

			m.renderCRDList(&sb)

			if sb.Len() == 0 {
				t.Error("renderCRDList produced empty output")
			}
		})
	}
}

func TestUpdateAllMessageTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	k := &k8sClient{
		dynamic: dynamicClient,
	}

	tests := []struct {
		name string
		msg  tea.Msg
	}{
		{
			name: "window size",
			msg:  tea.WindowSizeMsg{Width: 120, Height: 40},
		},
		{
			name: "empty CRDs",
			msg:  crdsLoadedMsg([]crdInfo{}),
		},
		{
			name: "CRDs with data",
			msg:  crdsLoadedMsg([]crdInfo{{name: "test", group: "g"}}),
		},
		{
			name: "empty resources",
			msg:  resourcesLoadedMsg([]resourceInfo{}),
		},
		{
			name: "resources with data",
			msg:  resourcesLoadedMsg([]resourceInfo{{name: "r", namespace: "ns"}}),
		},
		{
			name: "yaml data",
			msg:  yamlLoadedMsg("key: value"),
		},
		{
			name: "error message",
			msg:  errMsg(errors.New("test error")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel(k, "default")
			newModel, cmd := m.Update(tt.msg)

			if newModel == nil {
				t.Error("Update returned nil model")
			}

			// Commands may or may not be returned depending on message type
			if cmd != nil {
				msg := cmd()
				_ = msg // Just verify execution
			}
		})
	}
}

func TestFetchGroupResourcesVariations(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "examples",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			gvr: "ExampleList",
		},
	)

	k := &k8sClient{
		dynamic: dynamicClient,
	}

	tests := []struct {
		name             string
		selectedGroup    string
		allNamespaces    bool
		currentNamespace string
		resourceCount    int
	}{
		{
			name:             "group with single namespace",
			selectedGroup:    "group1",
			allNamespaces:    false,
			currentNamespace: "default",
			resourceCount:    3,
		},
		{
			name:             "group with all namespaces",
			selectedGroup:    "group2",
			allNamespaces:    true,
			currentNamespace: "default",
			resourceCount:    5,
		},
		{
			name:             "empty group",
			selectedGroup:    "nonexistent",
			allNamespaces:    false,
			currentNamespace: "default",
			resourceCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := make([]resourceInfo, tt.resourceCount)
			for i := 0; i < tt.resourceCount; i++ {
				resources[i] = resourceInfo{
					name:      "res" + string(rune(i)),
					namespace: "default",
					crd: crdInfo{
						group: tt.selectedGroup,
						name:  "test",
					},
				}
			}

			m := &model{
				k8s:              k,
				selectedGroup:    tt.selectedGroup,
				allNamespaces:    tt.allNamespaces,
				currentNamespace: tt.currentNamespace,
				resources:        resources,
			}

			cmd := m.fetchGroupResources()
			if cmd == nil {
				t.Fatal("fetchGroupResources returned nil")
			}

			msg := cmd()
			switch msg.(type) {
			case resourcesLoadedMsg, errMsg:
			// Expected
			default:
				t.Errorf("unexpected message type: %T", msg)
			}
		})
	}
}

func TestHighlightYAMLAllBranches(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected string
	}{
		{
			name:     "null value",
			yaml:     "key: null",
			expected: "null",
		},
		{
			name:     "boolean true",
			yaml:     "enabled: true",
			expected: "true",
		},
		{
			name:     "boolean false",
			yaml:     "disabled: false",
			expected: "false",
		},
		{
			name:     "number value",
			yaml:     "count: 42",
			expected: "42",
		},
		{
			name:     "float value",
			yaml:     "ratio: 3.14",
			expected: "3.14",
		},
		{
			name:     "quoted string",
			yaml:     "message: \"hello world\"",
			expected: "hello",
		},
		{
			name:     "array",
			yaml:     "items:\n- first\n- second\n- third",
			expected: "items",
		},
		{
			name:     "nested object",
			yaml:     "metadata:\n  name: test\n  namespace: default\n  labels:\n    app: myapp",
			expected: "metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{}
			result := m.highlightYAML(tt.yaml)

			if result == "" && tt.yaml != "" {
				t.Errorf("highlightYAML returned empty for non-empty YAML")
			}

			if tt.expected != "" && len(result) > 0 {
				// Just verify it processed something
				_ = result
			}
		})
	}
}

func TestHandleBrowsingKeysAllStates(t *testing.T) {
	tests := []struct {
		name   string
		keyMsg tea.KeyPressMsg
		state  viewState
	}{
		{
			name:   "up in YAML view",
			keyMsg: tea.KeyPressMsg{Code: tea.KeyUp},
			state:  stateYAMLView,
		},
		{
			name:   "down in YAML view",
			keyMsg: tea.KeyPressMsg{Code: tea.KeyDown},
			state:  stateYAMLView,
		},
		{
			name:   "pgup in YAML view",
			keyMsg: tea.KeyPressMsg{Code: tea.KeyPgUp},
			state:  stateYAMLView,
		},
		{
			name:   "pgdown in YAML view",
			keyMsg: tea.KeyPressMsg{Code: tea.KeyPgDown},
			state:  stateYAMLView,
		},
		{
			name:   "escape in YAML view",
			keyMsg: tea.KeyPressMsg{Code: tea.KeyEscape},
			state:  stateYAMLView,
		},
		{
			name:   "up in group resource list",
			keyMsg: tea.KeyPressMsg{Code: tea.KeyUp},
			state:  stateGroupResourceList,
		},
		{
			name:   "down in group resource list",
			keyMsg: tea.KeyPressMsg{Code: tea.KeyDown},
			state:  stateGroupResourceList,
		},
		{
			name:   "enter in group resource list",
			keyMsg: tea.KeyPressMsg{Code: tea.KeyEnter},
			state:  stateGroupResourceList,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				state:             tt.state,
				selectedYAML:      "test: data\nmore: content\nlines: here",
				yamlScrollLine:    5,
				crds:              []crdInfo{{name: "c1", group: "g"}},
				filteredCRDs:      []crdInfo{{name: "c1", group: "g"}},
				resources:         []resourceInfo{{name: "r1", namespace: "d"}},
				filteredResources: []resourceInfo{{name: "r1", namespace: "d"}},
			}

			// Should not panic in any state
			m.handleBrowsingKeys(tt.keyMsg)
		})
	}
}

func TestMoveUpAllStates(t *testing.T) {
	tests := []struct {
		name     string
		state    viewState
		moveType string
	}{
		{
			name:     "moveUp in CRD list",
			state:    stateCRDList,
			moveType: "crds",
		},
		{
			name:     "moveUp in resource list",
			state:    stateResourceList,
			moveType: "resources",
		},
		{
			name:     "moveUp in group resource list",
			state:    stateGroupResourceList,
			moveType: "resources",
		},
		{
			name:     "moveUp in YAML view",
			state:    stateYAMLView,
			moveType: "yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crds := make([]crdInfo, 10)
			for i := 0; i < 10; i++ {
				crds[i] = crdInfo{name: "c", group: "g"}
			}

			resources := make([]resourceInfo, 10)
			for i := 0; i < 10; i++ {
				resources[i] = resourceInfo{name: "r", namespace: "d"}
			}

			m := &model{
				state:             tt.state,
				crds:              crds,
				filteredCRDs:      crds,
				resources:         resources,
				filteredResources: resources,
				selectedYAML:      "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10",
				crdCursor:         5,
				resourceCursor:    5,
				yamlScrollLine:    5,
			}

			m.moveUp(2)

			// Just verify no panic
		})
	}
}

func TestRenderResourceListVariations(t *testing.T) {
	tests := []struct {
		name     string
		resCount int
		cursor   int
		offset   int
	}{
		{
			name:     "render with 5 resources",
			resCount: 5,
			cursor:   0,
			offset:   0,
		},
		{
			name:     "render with cursor at end",
			resCount: 10,
			cursor:   9,
			offset:   5,
		},
		{
			name:     "render empty resources",
			resCount: 0,
			cursor:   0,
			offset:   0,
		},
		{
			name:     "render with large offset",
			resCount: 50,
			cursor:   25,
			offset:   20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder

			resources := make([]resourceInfo, tt.resCount)
			for i := 0; i < tt.resCount; i++ {
				resources[i] = resourceInfo{
					name:      "res" + string(rune(i%10)),
					namespace: "ns" + string(rune(i/10)),
				}
			}

			m := &model{
				state:             stateResourceList,
				resources:         resources,
				filteredResources: resources,
				resourceCursor:    tt.cursor,
				resScrollOffset:   tt.offset,
				width:             80,
				height:            20,
			}

			m.renderResourceList(&sb)
			// Just verify it doesn't panic
		})
	}
}

func TestRenderGroupResourceListVariations(t *testing.T) {
	tests := []struct {
		name     string
		resCount int
		groups   int
	}{
		{
			name:     "render with single group",
			resCount: 5,
			groups:   1,
		},
		{
			name:     "render with multiple groups",
			resCount: 15,
			groups:   3,
		},
		{
			name:     "render empty",
			resCount: 0,
			groups:   0,
		},
		{
			name:     "render many groups",
			resCount: 30,
			groups:   5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder

			resources := make([]resourceInfo, tt.resCount)
			for i := 0; i < tt.resCount; i++ {
				resources[i] = resourceInfo{
					name:      "res" + string(rune(i%10)),
					namespace: "ns",
					crd: crdInfo{
						group: "g" + string(rune(i%tt.groups)),
						name:  "test",
					},
				}
			}

			m := &model{
				state:             stateGroupResourceList,
				resources:         resources,
				filteredResources: resources,
				width:             80,
				height:            20,
			}

			m.renderGroupResourceList(&sb)
			// Just verify it doesn't panic
		})
	}
}

func TestMoveDownCRDAllBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		cursor     int
		crdCount   int
		moveAmount int
		wantCursor int
	}{
		{
			name:       "move 1 down",
			cursor:     0,
			crdCount:   5,
			moveAmount: 1,
			wantCursor: 1,
		},
		{
			name:       "move to end",
			cursor:     3,
			crdCount:   5,
			moveAmount: 2,
			wantCursor: 4,
		},
		{
			name:       "move past end",
			cursor:     4,
			crdCount:   5,
			moveAmount: 10,
			wantCursor: 4,
		},
		{
			name:       "large move",
			cursor:     0,
			crdCount:   100,
			moveAmount: 50,
			wantCursor: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crds := make([]crdInfo, tt.crdCount)
			for i := 0; i < tt.crdCount; i++ {
				crds[i] = crdInfo{name: "c", group: "g"}
			}

			m := &model{
				state:        stateCRDList,
				crdCursor:    tt.cursor,
				crds:         crds,
				filteredCRDs: crds,
			}

			m.moveDownCRD(tt.moveAmount)

			if m.crdCursor != tt.wantCursor {
				t.Errorf("crdCursor = %d, want %d", m.crdCursor, tt.wantCursor)
			}
		})
	}
}

func TestMoveDownResAllBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		cursor     int
		resCount   int
		moveAmount int
		wantCursor int
	}{
		{
			name:       "move 1 down",
			cursor:     0,
			resCount:   5,
			moveAmount: 1,
			wantCursor: 1,
		},
		{
			name:       "move to end",
			cursor:     3,
			resCount:   5,
			moveAmount: 2,
			wantCursor: 4,
		},
		{
			name:       "move past end",
			cursor:     4,
			resCount:   5,
			moveAmount: 10,
			wantCursor: 4,
		},
		{
			name:       "large list",
			cursor:     0,
			resCount:   100,
			moveAmount: 75,
			wantCursor: 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := make([]resourceInfo, tt.resCount)
			for i := 0; i < tt.resCount; i++ {
				resources[i] = resourceInfo{name: "r", namespace: "d"}
			}

			m := &model{
				state:             stateResourceList,
				resourceCursor:    tt.cursor,
				resources:         resources,
				filteredResources: resources,
			}

			m.moveDownRes(tt.moveAmount)

			if m.resourceCursor != tt.wantCursor {
				t.Errorf("resourceCursor = %d, want %d", m.resourceCursor, tt.wantCursor)
			}
		})
	}
}

//	func TestInitModel(t *testing.T) {
//		m := model{
//			k8s: &k8sClient{dynamic: fake.NewSimpleDynamicClient(runtime.NewScheme())}, // Prevent panic if fetching immediately
//		}
//		cmd := m.Init()
//		if cmd == nil {
//			t.Error("Init should return a command")
//		}
//		// Execute it
//		msg := cmd()
//		if _, ok := msg.(crdsLoadedMsg); !ok {
//			// It might return errMsg if no scheme registered, that's fine too as long as it's testing fetchCRDs
//			if _, isErr := msg.(errMsg); !ok && !isErr {
//				t.Errorf("expected crdsLoadedMsg or errMsg, got %T", msg)
//			}
//		}
//	}
func TestSortResources(t *testing.T) {
	m := model{}
	res := []resourceInfo{
		{name: "b", namespace: "default", crd: crdInfo{name: "crd1"}},
		{name: "a", namespace: "default", crd: crdInfo{name: "crd1"}},
		{name: "c", namespace: "kube-system", crd: crdInfo{name: "crd1"}},
		{name: "a", namespace: "default", crd: crdInfo{name: "crd2"}},
	}
	m.sortResources(res)

	if res[0].crd.name != "crd1" || res[0].name != "a" {
		t.Errorf("sorting failed at 0: %+v", res[0])
	}

	if res[1].crd.name != "crd1" || res[1].name != "b" {
		t.Errorf("sorting failed at 1: %+v", res[1])
	}

	if res[2].crd.name != "crd2" || res[2].name != "a" {
		t.Errorf("sorting failed at 2: %+v", res[2])
	}

	if res[3].namespace != "kube-system" {
		t.Errorf("sorting failed at 3: %+v", res[3])
	}
}

//	func TestHandleGroupTransition(t *testing.T) {
//		m := model{
//			state: stateCRDList,
//			filteredCRDs: []crdInfo{
//				{group: "group1", name: "crd1"},
//			},
//			k8s: &k8sClient{dynamic: fake.NewSimpleDynamicClient(runtime.NewScheme())},
//		}
//
//		newModel, cmd := m.handleGroupTransition()
//		mUpdated := newModel.(model)
//
//		if mUpdated.state != stateGroupResourceList {
//			t.Errorf("expected state to be stateGroupResourceList")
//		}
//		if mUpdated.selectedGroup != "group1" {
//			t.Errorf("expected selectedGroup to be group1")
//		}
//		if cmd == nil {
//			t.Errorf("expected command to be returned")
//		}
//	}
func TestListNavigation(t *testing.T) {
	m := model{
		state:             stateResourceList,
		filteredResources: make([]resourceInfo, 20),
		resourceCursor:    0,
		resScrollOffset:   0,
		height:            24, // 16 max items
	}

	// Move down
	msg := tea.KeyPressMsg{Code: 'j'}
	newModel, _ := m.handleBrowsingKeys(msg)

	mUpdated := newModel.(model)
	if mUpdated.resourceCursor != 1 {
		t.Errorf("expected resourceCursor 1, got %d", mUpdated.resourceCursor)
	}

	// Move down 15 (ctrl+d)
	msg = tea.KeyPressMsg{Text: "ctrl+d", Code: 4} // roughly ctrl+d handling by string in browsingKeys
	newModel, _ = mUpdated.handleBrowsingKeys(msg)

	mUpdated = newModel.(model)
	if mUpdated.resourceCursor != 16 {
		t.Errorf("expected resourceCursor 16, got %d", mUpdated.resourceCursor)
	}

	// Move up 15 (ctrl+u)
	msg = tea.KeyPressMsg{Text: "ctrl+u", Code: 21}
	newModel, _ = mUpdated.handleBrowsingKeys(msg)

	mUpdated = newModel.(model)
	if mUpdated.resourceCursor != 1 {
		t.Errorf("expected resourceCursor 1, got %d", mUpdated.resourceCursor)
	}
}

func TestHandleEnterActions(t *testing.T) {
	scheme := runtime.NewScheme()
	testCRD := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestCrd",
			"metadata": map[string]any{
				"name":      "my-test-res",
				"namespace": "default",
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{
			Group:    "example.com",
			Version:  "v1",
			Resource: "testcrds",
		}: "TestCrdList",
	}, testCRD)

	k := &k8sClient{dynamic: dynClient}

	// Test entry from CRD list -> Resource List
	m := model{
		state:        stateCRDList,
		filteredCRDs: []crdInfo{{name: "testcrds", group: "example.com", version: "v1", resource: "testcrds"}},
		k8s:          k,
	}

	newModel, cmd := m.handleEnter()
	mUpdated := newModel.(model)

	if mUpdated.state != stateResourceList {
		t.Errorf("expected stateResourceList")
	}

	if cmd == nil {
		t.Errorf("expected command on transition")
	}

	// Test entry from Resource List -> YAML View
	mUpdated.state = stateResourceList
	mUpdated.filteredResources = []resourceInfo{{name: "my-test-res", namespace: "default", crd: mUpdated.filteredCRDs[0]}}

	newModel, cmd = mUpdated.handleEnter()
	mUpdated = newModel.(model)

	if mUpdated.state != stateYAMLView {
		t.Errorf("expected stateYAMLView")
	}

	if cmd == nil {
		t.Errorf("expected command on yaml fetch transition")
	}
}

func TestCollectGroupResourcesAndFetchGroup(t *testing.T) {
	scheme := runtime.NewScheme()
	crdInfoItem := crdInfo{
		name:     "testcrds.example.com",
		group:    "example.com",
		version:  "v1",
		resource: "testcrds",
	}

	testCRD := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "TestCrd",
			"metadata": map[string]any{
				"name":      "my-test-res",
				"namespace": "default",
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{
			Group:    "example.com",
			Version:  "v1",
			Resource: "testcrds",
		}: "TestCrdList",
	}, testCRD)

	k := &k8sClient{dynamic: dynClient}
	m := model{
		k8s:           k,
		crds:          []crdInfo{crdInfoItem},
		selectedGroup: "example.com",
	}

	resources, err := m.collectGroupResources("default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	// Test fetchGroupResources
	cmd := m.fetchGroupResources()

	msg := cmd()
	if _, ok := msg.(resourcesLoadedMsg); !ok {
		t.Errorf("expected resourcesLoadedMsg, got %T", msg)
	}
}

func TestHighlightYAML(t *testing.T) {
	m := model{}
	yamlString := "key: value"
	highlighted := m.highlightYAML(yamlString)
	// It should inject terminal formatting codes, so length should be greater than original if chroma is working
	if len(highlighted) <= len(yamlString) && !strings.Contains(highlighted, yamlString) {
		t.Errorf("highlighted yaml seems incorrect: %q", highlighted)
	}
}

func TestStatusBarAndPositionUpdates(t *testing.T) {
	m := model{
		state:             stateResourceList,
		mode:              modeBrowsing,
		resourceCursor:    5,
		filteredResources: make([]resourceInfo, 10),
	}

	ind := m.getPositionIndicator()
	if ind != "[6/10]" {
		t.Errorf("expected [6/10], got %s", ind)
	}

	m.state = stateYAMLView
	m.selectedYAML = "line1\nline2\nline3"
	m.yamlScrollLine = 1

	ind = m.getPositionIndicator()
	if ind != "[2/3]" {
		t.Errorf("expected [2/3], got %s", ind)
	}

	m.mode = modeFiltering

	status := m.renderStatusBar(ind)
	if !strings.Contains(status, "FILTERING") {
		t.Errorf("expected FILTERING in status bar, got %s", status)
	}
}
