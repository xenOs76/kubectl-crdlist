package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

// Additional tests for remaining coverage gaps
func TestHighlightYAMLBranches(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"null value", "key: null"},
		{"boolean", "enabled: true"},
		{"number", "count: 42"},
		{"float", "ratio: 3.14"},
		{"quoted", "msg: \"hello\""},
		{"nested", "obj:\n  key: val\n  sub: val2"},
		{"array", "items:\n  - a\n  - b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{}
			result := m.highlightYAML(tt.yaml)
			// Just verify it doesn't panic and returns something for non-empty input
			if tt.yaml != "" && len(result) == 0 {
				t.Error("highlightYAML returned empty for non-empty YAML")
			}
		})
	}
}

func TestMoveUpBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		state      viewState
		cursor     int
		total      int
		moveAmount int
	}{
		{"crd top", stateCRDList, 0, 10, 5},
		{"crd mid", stateCRDList, 5, 10, 2},
		{"res top", stateResourceList, 0, 5, 1},
		{"res mid", stateResourceList, 3, 5, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			crds := make([]crdInfo, tt.total)
			for i := 0; i < tt.total; i++ {
				crds[i] = crdInfo{name: "c", group: "g"}
			}

			resources := make([]resourceInfo, tt.total)
			for i := 0; i < tt.total; i++ {
				resources[i] = resourceInfo{name: "r", namespace: "d"}
			}

			m := &model{
				state:             tt.state,
				crdCursor:         tt.cursor,
				crds:              crds,
				filteredCRDs:      crds,
				resourceCursor:    tt.cursor,
				resources:         resources,
				filteredResources: resources,
			}

			m.moveUp(tt.moveAmount)
			// Just verify no panic
		})
	}
}

func TestFetchYAMLEdgeCases(t *testing.T) {
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
			name: "missing resource",
			crd:  crdInfo{name: "test.example.com", group: "example.com", version: "v1", resource: "tests"},
			res:  resourceInfo{name: "nonexistent", namespace: "default"},
		},
		{
			name: "different namespace",
			crd:  crdInfo{name: "test.example.com", group: "example.com", version: "v1", resource: "tests"},
			res:  resourceInfo{name: "missing", namespace: "kube-system"},
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
				t.Fatalf("unexpected message type: %T", msg)
			}
		})
	}
}

func TestBrowsingKeysNavigationAll(t *testing.T) {
	tests := []struct {
		name   string
		keyMsg tea.KeyPressMsg
		state  viewState
	}{
		{"up crd", tea.KeyPressMsg{Code: tea.KeyUp}, stateCRDList},
		{"down crd", tea.KeyPressMsg{Code: tea.KeyDown}, stateCRDList},
		{"up res", tea.KeyPressMsg{Code: tea.KeyUp}, stateResourceList},
		{"down res", tea.KeyPressMsg{Code: tea.KeyDown}, stateResourceList},
		{"pgup", tea.KeyPressMsg{Code: tea.KeyPgUp}, stateCRDList},
		{"pgdown", tea.KeyPressMsg{Code: tea.KeyPgDown}, stateCRDList},
		{"home", tea.KeyPressMsg{Code: tea.KeyHome}, stateResourceList},
		{"end", tea.KeyPressMsg{Code: tea.KeyEnd}, stateResourceList},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
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

			m.handleBrowsingKeys(tt.keyMsg)
		})
	}
}

func TestRenderingEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		itemCount int
		state     viewState
	}{
		{"empty crds", 0, stateCRDList},
		{"single crd", 1, stateCRDList},
		{"many crds", 20, stateCRDList},
		{"empty resources", 0, stateResourceList},
		{"single resource", 1, stateResourceList},
		{"many resources", 15, stateResourceList},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			var sb strings.Builder

			crds := make([]crdInfo, tt.itemCount)
			for i := 0; i < tt.itemCount; i++ {
				crds[i] = crdInfo{name: "c", group: "g"}
			}

			resources := make([]resourceInfo, tt.itemCount)
			for i := 0; i < tt.itemCount; i++ {
				resources[i] = resourceInfo{name: "r", namespace: "d"}
			}

			m := &model{
				state:             tt.state,
				crds:              crds,
				filteredCRDs:      crds,
				resources:         resources,
				filteredResources: resources,
				width:             80,
				height:            20,
			}

			switch tt.state {
			case stateCRDList:
				m.renderCRDList(&sb)
			case stateResourceList:
				m.renderResourceList(&sb)
			default:
				// No rendering for other states in this test
			}
		})
	}
}

func TestMoveDownCRDDirect(t *testing.T) {
	tests := []struct {
		name   string
		count  int
		cursor int
		move   int
		want   int
	}{
		{"crd 1", 5, 0, 1, 1},
		{"crd end", 5, 3, 2, 4},
		{"crd over", 5, 4, 10, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crds := make([]crdInfo, tt.count)
			m := &model{crdCursor: tt.cursor, crds: crds, filteredCRDs: crds}
			m.moveDownCRD(tt.move)

			if m.crdCursor != tt.want {
				t.Errorf("cursor=%d want=%d", m.crdCursor, tt.want)
			}
		})
	}
}

func TestMoveDownResDirect(t *testing.T) {
	tests := []struct {
		name   string
		count  int
		cursor int
		move   int
		want   int
	}{
		{"res 1", 5, 0, 1, 1},
		{"res end", 5, 3, 2, 4},
		{"res over", 5, 4, 10, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := make([]resourceInfo, tt.count)
			m := &model{resourceCursor: tt.cursor, resources: resources, filteredResources: resources}
			m.moveDownRes(tt.move)

			if m.resourceCursor != tt.want {
				t.Errorf("cursor=%d want=%d", m.resourceCursor, tt.want)
			}
		})
	}
}

func TestHandleBrowsingKeysComprehensive(t *testing.T) {
	tests := []struct {
		name   string
		keyMsg tea.KeyPressMsg
		state  viewState
	}{
		{"crd up", tea.KeyPressMsg{Code: tea.KeyUp}, stateCRDList},
		{"crd down", tea.KeyPressMsg{Code: tea.KeyDown}, stateCRDList},
		{"crd pgup", tea.KeyPressMsg{Code: tea.KeyPgUp}, stateCRDList},
		{"crd pgdn", tea.KeyPressMsg{Code: tea.KeyPgDown}, stateCRDList},
		{"crd home", tea.KeyPressMsg{Code: tea.KeyHome}, stateCRDList},
		{"crd end", tea.KeyPressMsg{Code: tea.KeyEnd}, stateCRDList},
		{"res up", tea.KeyPressMsg{Code: tea.KeyUp}, stateResourceList},
		{"res down", tea.KeyPressMsg{Code: tea.KeyDown}, stateResourceList},
		{"yaml up", tea.KeyPressMsg{Code: tea.KeyUp}, stateYAMLView},
		{"yaml down", tea.KeyPressMsg{Code: tea.KeyDown}, stateYAMLView},
		{"filter", tea.KeyPressMsg{Text: "/", Code: '/'}, stateCRDList},
		{"esc", tea.KeyPressMsg{Code: tea.KeyEscape}, stateCRDList},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
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
				selectedYAML: "test\ndata\nhere",
			}

			m.handleBrowsingKeys(tt.keyMsg)
		})
	}
}

// func TestHandleEnterAllStates(t *testing.T) {
// 	scheme := runtime.NewScheme()
// 	gvr := schema.GroupVersionResource{
// 		Group:    "example.com",
// 		Version:  "v1",
// 		Resource: "examples",
// 	}
//
// 	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
// 		scheme,
// 		map[schema.GroupVersionResource]string{
// 			gvr: "ExampleList",
// 		},
// 	)
//
// 	k := &k8sClient{
// 		dynamic: dynamicClient,
// 	}
//
// 	tests := []struct {
// 		name  string
// 		state viewState
// 	}{
// 		{"crd list", stateCRDList},
// 		{"resource list", stateResourceList},
// 		{"group resource list", stateGroupResourceList},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			m := &model{
// 				state: tt.state,
// 				crds: []crdInfo{
// 					{name: "c1", group: "g", version: "v1", resource: "tests"},
// 				},
// 				filteredCRDs: []crdInfo{
// 					{name: "c1", group: "g", version: "v1", resource: "tests"},
// 				},
// 				resources: []resourceInfo{
// 					{name: "r1", namespace: "default"},
// 				},
// 				filteredResources: []resourceInfo{
// 					{name: "r1", namespace: "default"},
// 				},
// 				k8s: k,
// 			}
//
// 			newModel, cmd := m.handleEnter()
// 			if newModel == nil {
// 				t.Error("handleEnter returned nil model")
// 			}
//
// 			if cmd != nil {
// 				msg := cmd()
// 				_ = msg
// 			}
// 		})
// 	}
// }

// func TestRenderYAMLViewScrolling(t *testing.T) {
// 	tests := []struct {
// 		name          string
// 		scrollLine    int
// 		yamlLineCount int
// 		height        int
// 	}{
// 		{"scroll top", 0, 10, 5},
// 		{"scroll mid", 3, 10, 5},
// 		{"scroll end", 9, 10, 5},
// 		{"small yaml", 0, 3, 10},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			var sb strings.Builder
//
// 			lines := make([]string, tt.yamlLineCount)
// 			for i := 0; i < tt.yamlLineCount; i++ {
// 				lines[i] = "line " + string(rune('0'+byte(i%10)))
// 			}
//
// 			yaml := strings.Join(lines, "\n")
//
// 			m := &model{
// 				state:          stateYAMLView,
// 				selectedYAML:   yaml,
// 				yamlScrollLine: tt.scrollLine,
// 				width:          80,
// 				height:         tt.height,
// 			}
//
// 			m.renderYAMLView(&sb)
//
// 			if sb.Len() == 0 {
// 				t.Error("renderYAMLView produced empty output")
// 			}
// 		})
// 	}
// }

func TestHandleGroupTransition(t *testing.T) {
	tests := []struct {
		name         string
		resources    []resourceInfo
		wantGroupSet bool
	}{
		{
			name: "with resources",
			resources: []resourceInfo{
				{name: "r1", crd: crdInfo{group: "g1"}},
				{name: "r2", crd: crdInfo{group: "g2"}},
			},
			wantGroupSet: true,
		},
		{
			name:         "no resources",
			resources:    []resourceInfo{},
			wantGroupSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model{
				state:     stateResourceList,
				resources: tt.resources,
			}

			newModel, cmd := m.handleGroupTransition()
			if newModel == nil {
				t.Error("handleGroupTransition returned nil")
			}

			if cmd != nil {
				msg := cmd()
				_ = msg
			}
		})
	}
}

func TestRenderFunctionsWithVariousInputs(t *testing.T) {
	tests := []struct {
		name   string
		crds   int
		res    int
		cursor int
		state  viewState
	}{
		{"1 crd", 1, 0, 0, stateCRDList},
		{"10 crds", 10, 0, 5, stateCRDList},
		{"1 res", 0, 1, 0, stateResourceList},
		{"10 res", 0, 10, 7, stateResourceList},
		{"group res", 0, 20, 0, stateGroupResourceList},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder

			crds := make([]crdInfo, tt.crds)
			for i := 0; i < tt.crds; i++ {
				crds[i] = crdInfo{name: "c" + string(rune('0'+byte(i))), group: "g"}
			}

			resources := make([]resourceInfo, tt.res)
			for i := 0; i < tt.res; i++ {
				resources[i] = resourceInfo{name: "r" + string(rune('0'+byte(i))), namespace: "d"}
			}

			m := &model{
				state:             tt.state,
				crds:              crds,
				filteredCRDs:      crds,
				resources:         resources,
				filteredResources: resources,
				crdCursor:         tt.cursor % (tt.crds + 1),
				resourceCursor:    tt.cursor % (tt.res + 1),
				width:             80,
				height:            20,
			}

			switch tt.state {
			case stateCRDList:
				m.renderCRDList(&sb)
			case stateResourceList:
				m.renderResourceList(&sb)
			case stateGroupResourceList:
				m.renderGroupResourceList(&sb)
			default:
				// No rendering for other states
			}

			if sb.Len() == 0 && tt.crds+tt.res > 0 {
				t.Error("render returned empty output for non-empty data")
			}
		})
	}
}
