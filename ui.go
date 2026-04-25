package main

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#89b4fa")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#cba6f7")).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#585b70"))

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("#585b70")).
			Foreground(lipgloss.Color("#f5e0dc"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6c7086"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f38ba8"))

	statusStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#313244")).
			Foreground(lipgloss.Color("#bac2de")).
			Padding(0, 1)

	browsingModeStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#89b4fa")).
				Foreground(lipgloss.Color("#1e1e2e")).
				Padding(0, 1).
				Bold(true)

	filteringModeStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#f9e2af")).
				Foreground(lipgloss.Color("#1e1e2e")).
				Padding(0, 1).
				Bold(true)
)

func (m model) Init() tea.Cmd {
	return m.fetchCRDs()
}

func (m model) fetchCRDs() tea.Cmd {
	return func() tea.Msg {
		crds, err := m.k8s.listCRDs()
		if err != nil {
			return errMsg(err)
		}

		return crdsLoadedMsg(crds)
	}
}

func (m model) fetchResources() tea.Cmd {
	return func() tea.Msg {
		ns := m.currentNamespace
		if m.allNamespaces {
			ns = ""
		}

		res, err := m.k8s.listResources(m.selectedCRD, ns)
		if err != nil {
			return errMsg(err)
		}

		return resourcesLoadedMsg(res)
	}
}

func (m model) fetchGroupResources() tea.Cmd {
	return func() tea.Msg {
		ns := m.currentNamespace
		if m.allNamespaces {
			ns = ""
		}

		aggregated, err := m.collectGroupResources(ns)
		if err != nil {
			return errMsg(err)
		}

		m.sortResources(aggregated)

		return resourcesLoadedMsg(aggregated)
	}
}

func (m model) collectGroupResources(ns string) ([]resourceInfo, error) {
	var aggregated []resourceInfo

	for _, crd := range m.crds {
		if crd.group == m.selectedGroup {
			res, err := m.k8s.listResources(crd, ns)
			if err != nil {
				return nil, err
			}

			aggregated = append(aggregated, res...)
		}
	}

	return aggregated, nil
}

func (model) sortResources(res []resourceInfo) {
	sort.Slice(res, func(i, j int) bool {
		if res[i].namespace != res[j].namespace {
			return res[i].namespace < res[j].namespace
		}

		if res[i].crd.name != res[j].crd.name {
			return res[i].crd.name < res[j].crd.name
		}

		return res[i].name < res[j].name
	})
}

func (m model) fetchYAML() tea.Cmd {
	return func() tea.Msg {
		yaml, err := m.k8s.fetchResourceYAML(m.selectedCRD, m.selectedRes.name, m.selectedRes.namespace)
		if err != nil {
			return errMsg(err)
		}

		return yamlLoadedMsg(yaml)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case crdsLoadedMsg:
		m.crds = msg
		sort.Slice(m.crds, func(i, j int) bool {
			return m.crds[i].group < m.crds[j].group
		})
		m.filteredCRDs = m.crds
		m.loading = false
		m.crdCursor = 0

	case resourcesLoadedMsg:
		m.resources = msg
		m.filteredResources = m.resources
		m.loading = false
		m.resourceCursor = 0
		m.resScrollOffset = 0

	case yamlLoadedMsg:
		m.selectedYAML = string(msg)
		m.loading = false
		m.yamlScrollLine = 0

	case errMsg:
		m.err = msg
		m.loading = false

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	}

	return m, cmd
}

func (m model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeFiltering {
		return m.handleFilteringKeys(msg)
	}

	return m.handleBrowsingKeys(msg)
}

func (m model) handleFilteringKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch k {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "enter": // Exit filtering mode
		m.mode = modeBrowsing
	case "backspace":
		m.handleBackspace()
	default:
		m.handleDefaultKey(k)
	}

	return m, nil
}

func (m model) handleBrowsingKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch k {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.handleEscape()
	case "up", "k", "ctrl+k":
		m.moveUp(1)
	case "down", "j", "ctrl+j":
		m.moveDown(1)
	case "ctrl+u":
		m.moveUp(15)
	case "ctrl+d":
		m.moveDown(15)
	case "enter":
		m.mode = modeBrowsing // Ensure browsing mode on transition
		return m.handleEnter()
	case "n":
		return m.handleNamespaceToggle()
	case "g":
		return m.handleGroupTransition()
	case "/":
		m.handleFilterActivation()
	default:
		// No default action for browsing keys
	}

	return m, nil
}

func (m model) handleGroupTransition() (tea.Model, tea.Cmd) {
	if m.state == stateCRDList && len(m.filteredCRDs) > 0 {
		m.selectedGroup = m.filteredCRDs[m.crdCursor].group
		m.state = stateGroupResourceList
		m.mode = modeBrowsing // Ensure browsing mode on transition
		m.loading = true

		return m, m.fetchGroupResources()
	}

	return m, nil
}

func (m *model) handleFilterActivation() {
	if m.state == stateCRDList || m.state == stateResourceList || m.state == stateGroupResourceList {
		m.mode = modeFiltering
	}
}

func (m model) handleNamespaceToggle() (tea.Model, tea.Cmd) {
	if m.state == stateResourceList {
		m.allNamespaces = !m.allNamespaces
		m.loading = true

		return m, m.fetchResources()
	}

	if m.state == stateGroupResourceList {
		m.allNamespaces = !m.allNamespaces
		m.loading = true

		return m, m.fetchGroupResources()
	}

	return m, nil
}

func (m *model) handleBackspace() {
	inListState := m.state == stateCRDList ||
		m.state == stateResourceList ||
		m.state == stateGroupResourceList

	if inListState && len(m.filter) > 0 {
		m.filter = m.filter[:len(m.filter)-1]
		m.applyFilter()
	}
}

func (m *model) handleDefaultKey(k string) {
	inListState := m.state == stateCRDList ||
		m.state == stateResourceList ||
		m.state == stateGroupResourceList

	if inListState && len(k) == 1 {
		m.filter += k
		m.applyFilter()
	}
}

func (m *model) handleEscape() {
	m.err = nil

	if m.mode == modeFiltering {
		m.mode = modeBrowsing

		return
	}

	if m.state == stateYAMLView {
		if m.selectedGroup != "" {
			m.state = stateGroupResourceList
		} else {
			m.state = stateResourceList
		}

		return
	}

	if m.state == stateResourceList || m.state == stateGroupResourceList {
		m.state = stateCRDList
		m.selectedGroup = ""
		m.filter = ""
		m.filteredCRDs = m.crds
		m.filteredResources = m.resources

		return
	}

	if m.filter != "" {
		m.filter = ""
		m.filteredCRDs = m.crds
		m.filteredResources = m.resources
		m.crdCursor = 0
		m.resourceCursor = 0
	}
}

func (m *model) moveUp(amount int) {
	switch m.state {
	case stateCRDList:
		m.crdCursor -= amount
		if m.crdCursor < 0 {
			m.crdCursor = 0
		}

		if m.crdCursor < m.crdScrollOffset {
			m.crdScrollOffset = m.crdCursor
		}
	case stateResourceList, stateGroupResourceList:
		m.resourceCursor -= amount
		if m.resourceCursor < 0 {
			m.resourceCursor = 0
		}

		if m.resourceCursor < m.resScrollOffset {
			m.resScrollOffset = m.resourceCursor
		}
	case stateYAMLView:
		m.yamlScrollLine -= amount
		if m.yamlScrollLine < 0 {
			m.yamlScrollLine = 0
		}
	default:
	}
}

func (m *model) moveDown(amount int) {
	switch m.state {
	case stateCRDList:
		m.moveDownCRD(amount)
	case stateResourceList, stateGroupResourceList:
		m.moveDownRes(amount)
	case stateYAMLView:
		lines := strings.Split(m.selectedYAML, "\n")

		m.yamlScrollLine += amount
		if m.yamlScrollLine > len(lines)-1 {
			m.yamlScrollLine = len(lines) - 1
		}

		if m.yamlScrollLine < 0 {
			m.yamlScrollLine = 0
		}
	default:
	}
}

func (m *model) moveDownCRD(amount int) {
	maxHeight := m.height - 8

	m.crdCursor += amount
	if m.crdCursor >= len(m.filteredCRDs) {
		m.crdCursor = len(m.filteredCRDs) - 1
	}

	if m.crdCursor < 0 {
		m.crdCursor = 0
	}

	if m.crdCursor >= m.crdScrollOffset+maxHeight {
		m.crdScrollOffset = m.crdCursor - maxHeight + 1
	}

	if m.crdScrollOffset < 0 {
		m.crdScrollOffset = 0
	}
}

func (m *model) moveDownRes(amount int) {
	maxHeight := m.height - 8

	m.resourceCursor += amount
	if m.resourceCursor >= len(m.filteredResources) {
		m.resourceCursor = len(m.filteredResources) - 1
	}

	if m.resourceCursor < 0 {
		m.resourceCursor = 0
	}

	if m.resourceCursor >= m.resScrollOffset+maxHeight {
		m.resScrollOffset = m.resourceCursor - maxHeight + 1
	}

	if m.resScrollOffset < 0 {
		m.resScrollOffset = 0
	}
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	if m.state == stateCRDList && len(m.filteredCRDs) > 0 {
		m.selectedCRD = m.filteredCRDs[m.crdCursor]
		m.selectedGroup = "" // Clear group if selecting single
		m.state = stateResourceList
		m.loading = true

		return m, m.fetchResources()
	}

	if (m.state == stateResourceList || m.state == stateGroupResourceList) && len(m.filteredResources) > 0 {
		m.selectedRes = m.filteredResources[m.resourceCursor]
		m.selectedCRD = m.selectedRes.crd // Sync CRD for fetchYAML
		m.state = stateYAMLView
		m.loading = true

		return m, m.fetchYAML()
	}

	return m, nil
}

func (m *model) applyFilter() {
	switch m.state {
	case stateCRDList:
		m.filteredCRDs = nil

		for _, crd := range m.crds {
			if strings.Contains(strings.ToLower(crd.name), strings.ToLower(m.filter)) {
				m.filteredCRDs = append(m.filteredCRDs, crd)
			}
		}

		m.crdCursor = 0
		m.crdScrollOffset = 0
	case stateResourceList, stateGroupResourceList:
		m.filteredResources = nil

		for _, res := range m.resources {
			if strings.Contains(strings.ToLower(res.name), strings.ToLower(m.filter)) {
				m.filteredResources = append(m.filteredResources, res)
			}
		}

		m.resourceCursor = 0
		m.resScrollOffset = 0
	default:
	}
}

func (m model) View() tea.View {
	if m.err != nil {
		return tea.NewView(errorStyle.Render(fmt.Sprintf("Error: %v\n\nPress 'esc' to go back, 'q' to quit.", m.err)))
	}

	if m.loading {
		return tea.NewView(dimStyle.Render("Loading data from cluster..."))
	}

	var s strings.Builder

	m.renderHeader(&s)

	switch m.state {
	case stateCRDList:
		m.renderCRDList(&s)
	case stateResourceList:
		m.renderResourceList(&s)
	case stateGroupResourceList:
		m.renderGroupResourceList(&s)
	case stateYAMLView:
		m.renderYAMLView(&s)
	default:
	}

	posIndicator := m.getPositionIndicator()

	s.WriteString("\n" + m.renderStatusBar(posIndicator))

	return tea.NewView(s.String())
}

func (m model) getPositionIndicator() string {
	var posIndicator string

	switch m.state {
	case stateCRDList:
		if len(m.filteredCRDs) > 0 {
			posIndicator = fmt.Sprintf("[%d/%d]", m.crdCursor+1, len(m.filteredCRDs))
		}
	case stateResourceList, stateGroupResourceList:
		if len(m.filteredResources) > 0 {
			posIndicator = fmt.Sprintf("[%d/%d]", m.resourceCursor+1, len(m.filteredResources))
		}
	case stateYAMLView:
		lines := strings.Split(m.selectedYAML, "\n")
		posIndicator = fmt.Sprintf("[%d/%d]", m.yamlScrollLine+1, len(lines))
	default:
	}

	return posIndicator
}

func (m model) renderStatusBar(posIndicator string) string {
	var legend string

	var modeIndicator string

	if m.mode == modeFiltering {
		modeIndicator = filteringModeStyle.Render(" FILTERING ")
		legend = "enter: confirm • esc: stop searching • backspace: delete"
	} else {
		modeIndicator = browsingModeStyle.Render(" BROWSING ")

		switch m.state {
		case stateResourceList, stateGroupResourceList:
			legend = "↑/↓, j/k, ^d/^u: navigate • enter: view • n: namespace • esc: back • q: quit"
		case stateYAMLView:
			legend = "↑/↓, j/k, ^d/^u: scroll • esc: back • q: quit"
		default:
			legend = "/: filter • ↑/↓, j/k, ^d/^u: navigate • enter: select • g: group • esc: back • q: quit"
		}
	}

	return modeIndicator + " " + dimStyle.Render(posIndicator) + " " + statusStyle.Render(legend)
}

func (m model) renderHeader(s *strings.Builder) {
	header := titleStyle.Render("kubectl-crdlist")

	switch m.state {
	case stateCRDList:
		header += " > " + dimStyle.Render("General")
	case stateResourceList:
		header += " > " + dimStyle.Render("CRD Resource: ") + m.selectedCRD.name
	case stateGroupResourceList:
		header += " > " + dimStyle.Render("CRD Group: ") + m.selectedGroup
	case stateYAMLView:
		header += " > " + dimStyle.Render("YAML: ") + m.selectedRes.name
	default:
	}

	s.WriteString(header + "\n\n")
}

func (m model) renderCRDList(s *strings.Builder) {
	cursor := ""
	if m.mode == modeFiltering {
		cursor = "█"
	}

	s.WriteString(dimStyle.Render("Filter: ") + m.filter + cursor + "\n\n")
	s.WriteString(headerStyle.Render(fmt.Sprintf("%-40s %-60s", "GROUP", "CRD NAME")))
	s.WriteString("\n")

	maxItems := m.height - 8
	start := m.crdScrollOffset
	end := start + maxItems

	if end > len(m.filteredCRDs) {
		end = len(m.filteredCRDs)
	}

	for i := start; i < end; i++ {
		crd := m.filteredCRDs[i]
		line := fmt.Sprintf("%-40s %-60s", crd.group, crd.name)

		if i == m.crdCursor {
			s.WriteString(selectedStyle.Render(line))
		} else {
			s.WriteString(line)
		}

		s.WriteString("\n")
	}
}

func (m model) renderResourceList(s *strings.Builder) {
	cursor := ""
	if m.mode == modeFiltering {
		cursor = "█"
	}

	s.WriteString(dimStyle.Render("Filter: ") + m.filter + cursor + "\n\n")

	nsDisplay := "namespace: " + m.currentNamespace
	if m.allNamespaces {
		nsDisplay = "all namespaces"
	}

	s.WriteString(dimStyle.Render("Viewing "+nsDisplay+" (Press 'n' to toggle)") + "\n\n")
	s.WriteString(headerStyle.Render(fmt.Sprintf("%-40s %-40s", "NAMESPACE", "NAME")))
	s.WriteString("\n")

	maxItems := m.height - 8
	start := m.resScrollOffset
	end := start + maxItems

	if end > len(m.filteredResources) {
		end = len(m.filteredResources)
	}

	for i := start; i < end; i++ {
		res := m.filteredResources[i]
		ns := res.namespace

		if ns == "" {
			ns = "-"
		}

		line := fmt.Sprintf("%-40s %-40s", ns, res.name)

		if i == m.resourceCursor {
			s.WriteString(selectedStyle.Render(line))
		} else {
			s.WriteString(line)
		}

		s.WriteString("\n")
	}
}

func (m model) renderGroupResourceList(s *strings.Builder) {
	cursor := ""
	if m.mode == modeFiltering {
		cursor = "█"
	}

	s.WriteString(dimStyle.Render("Filter: ") + m.filter + cursor + "\n\n")

	nsDisplay := "namespace: " + m.currentNamespace
	if m.allNamespaces {
		nsDisplay = "all namespaces"
	}

	s.WriteString(dimStyle.Render("Viewing "+nsDisplay+" (Press 'n' to toggle)") + "\n\n")
	s.WriteString(headerStyle.Render(fmt.Sprintf("%-20s %-40s %-40s", "NAMESPACE", "CRD NAME", "RESOURCE NAME")))
	s.WriteString("\n")

	maxItems := m.height - 8
	start := m.resScrollOffset
	end := start + maxItems

	if end > len(m.filteredResources) {
		end = len(m.filteredResources)
	}

	for i := start; i < end; i++ {
		res := m.filteredResources[i]
		ns := res.namespace

		if ns == "" {
			ns = "-"
		}

		line := fmt.Sprintf("%-20s %-40s %-40s", ns, res.crd.name, res.name)

		if i == m.resourceCursor {
			s.WriteString(selectedStyle.Render(line))
		} else {
			s.WriteString(line)
		}

		s.WriteString("\n")
	}
}

func (m model) renderYAMLView(s *strings.Builder) {
	highlighted := m.highlightYAML(m.selectedYAML)
	lines := strings.Split(highlighted, "\n")

	start := m.yamlScrollLine
	end := start + (m.height - 8)

	if end > len(lines) {
		end = len(lines)
	}

	if start < 0 {
		start = 0
	}

	for i := start; i < end; i++ {
		s.WriteString(lines[i] + "\n")
	}
}

func (model) highlightYAML(source string) string {
	lexer := lexers.Get("yaml")

	if lexer == nil {
		lexer = lexers.Fallback
	}

	style := styles.Get("catppuccin-mocha")

	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")

	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return source
	}

	var sb strings.Builder

	err = formatter.Format(&sb, style, iterator)
	if err != nil {
		return source
	}

	return sb.String()
}
