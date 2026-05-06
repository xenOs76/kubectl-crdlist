package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/xenos76/kubectl-crdlist/internal/model"
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

func (m Model) Init() tea.Cmd {
	return m.fetchCRDs()
}

func (m Model) fetchCRDs() tea.Cmd {
	return func() tea.Msg {
		crds, err := m.K8s.ListCRDs()
		if err != nil {
			return model.ErrMsg(err)
		}

		return model.CRDsLoadedMsg(crds)
	}
}

func (m Model) fetchResources() tea.Cmd {
	return func() tea.Msg {
		ns := m.CurrentNamespace
		if m.AllNamespaces {
			ns = ""
		}

		res, err := m.K8s.ListResources(m.SelectedCRD, ns)
		if err != nil {
			return model.ErrMsg(err)
		}

		return model.ResourcesLoadedMsg(res)
	}
}

func (m Model) fetchGroupResources() tea.Cmd {
	return func() tea.Msg {
		ns := m.CurrentNamespace
		if m.AllNamespaces {
			ns = ""
		}

		aggregated, err := m.collectGroupResources(ns)
		if err != nil {
			return model.ErrMsg(err)
		}

		m.sortResources(aggregated)

		return model.ResourcesLoadedMsg(aggregated)
	}
}

func (m Model) collectGroupResources(ns string) ([]model.ResourceInfo, error) {
	var aggregated []model.ResourceInfo

	for _, crd := range m.Crds {
		if crd.Group == m.SelectedGroup {
			res, err := m.K8s.ListResources(crd, ns)
			if err != nil {
				return nil, err
			}

			aggregated = append(aggregated, res...)
		}
	}

	return aggregated, nil
}

func (Model) sortResources(res []model.ResourceInfo) {
	slices.SortFunc(res, func(a, b model.ResourceInfo) int {
		if a.Namespace != b.Namespace {
			return cmp.Compare(a.Namespace, b.Namespace)
		}

		if a.CRD.Name != b.CRD.Name {
			return cmp.Compare(a.CRD.Name, b.CRD.Name)
		}

		return cmp.Compare(a.Name, b.Name)
	})
}

func (m Model) fetchYAML() tea.Cmd {
	return func() tea.Msg {
		yaml, err := m.K8s.FetchResourceYAML(m.SelectedCRD, m.SelectedRes.Name, m.SelectedRes.Namespace)
		if err != nil {
			return model.ErrMsg(err)
		}

		return model.YAMLLoadedMsg(yaml)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

	case model.CRDsLoadedMsg:
		m.Crds = msg
		slices.SortFunc(m.Crds, func(a, b model.CRDInfo) int {
			return cmp.Compare(a.Group, b.Group)
		})
		m.FilteredCRDs = m.Crds
		m.Loading = false
		m.CrdCursor = 0

	case model.ResourcesLoadedMsg:
		m.Resources = msg
		m.FilteredResources = m.Resources
		m.Loading = false
		m.ResourceCursor = 0
		m.ResScrollOffset = 0

	case model.YAMLLoadedMsg:
		m.SelectedYAML = string(msg)
		m.Loading = false
		m.YamlScrollLine = 0

	case model.ErrMsg:
		m.Err = msg
		m.Loading = false

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	default:
		// No-op for other messages
	}

	return m, cmd
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.Mode == model.ModeFiltering {
		return m.handleFilteringKeys(msg)
	}

	return m.handleBrowsingKeys(msg)
}

func (m Model) handleFilteringKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch k {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "enter": // Exit filtering mode
		m.Mode = model.ModeBrowsing
	case "backspace":
		m.handleBackspace()
	default:
		m.handleDefaultKey(k)
	}

	return m, nil
}

func (m Model) handleBrowsingKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch k {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.handleEscape()
	case "up", "k", "ctrl+k", "down", "j", "ctrl+j", "ctrl+u", "ctrl+d":
		m.handleNavigation(k)
	case "enter":
		m.Mode = model.ModeBrowsing // Ensure browsing mode on transition
		return m.handleEnter()
	case "n":
		return m.handleNamespaceToggle()
	case "g", "/":
		return m.handleActionKey(k)
	default:
		// No default action for browsing keys
	}

	return m, nil
}

func (m *Model) handleNavigation(k string) {
	switch k {
	case "up", "k", "ctrl+k":
		m.moveUp(1)
	case "down", "j", "ctrl+j":
		m.moveDown(1)
	case "ctrl+u":
		m.moveUp(15)
	case "ctrl+d":
		m.moveDown(15)
	default:
	}
}

func (m Model) handleActionKey(k string) (tea.Model, tea.Cmd) {
	if m.Err != nil {
		return m, nil
	}

	switch k {
	case "g":
		return m.handleGroupTransition()
	case "/":
		m.handleFilterActivation()
	default:
	}

	return m, nil
}

func (m Model) handleGroupTransition() (tea.Model, tea.Cmd) {
	if m.State == model.StateCRDList && len(m.FilteredCRDs) > 0 {
		m.SelectedGroup = m.FilteredCRDs[m.CrdCursor].Group
		m.State = model.StateGroupResourceList
		m.Mode = model.ModeBrowsing // Ensure browsing mode on transition
		m.Loading = true

		return m, m.fetchGroupResources()
	}

	return m, nil
}

func (m *Model) handleFilterActivation() {
	if m.State == model.StateCRDList || m.State == model.StateResourceList || m.State == model.StateGroupResourceList {
		m.Mode = model.ModeFiltering
	}
}

func (m Model) handleNamespaceToggle() (tea.Model, tea.Cmd) {
	m.Err = nil // Clear error on retry

	if m.State == model.StateResourceList {
		m.AllNamespaces = !m.AllNamespaces
		m.Loading = true

		return m, m.fetchResources()
	}

	if m.State == model.StateGroupResourceList {
		m.AllNamespaces = !m.AllNamespaces
		m.Loading = true

		return m, m.fetchGroupResources()
	}

	return m, nil
}

func (m *Model) handleBackspace() {
	inListState := m.State == model.StateCRDList ||
		m.State == model.StateResourceList ||
		m.State == model.StateGroupResourceList

	if inListState && len(m.Filter) > 0 {
		m.Filter = m.Filter[:len(m.Filter)-1]
		m.applyFilter()
	}
}

func (m *Model) handleDefaultKey(k string) {
	inListState := m.State == model.StateCRDList ||
		m.State == model.StateResourceList ||
		m.State == model.StateGroupResourceList

	if inListState && len(k) == 1 {
		m.Filter += k
		m.applyFilter()
	}
}

func (m *Model) handleEscape() {
	m.Err = nil

	if m.Mode == model.ModeFiltering {
		m.Mode = model.ModeBrowsing

		return
	}

	if m.State == model.StateYAMLView {
		if m.SelectedGroup != "" {
			m.State = model.StateGroupResourceList
		} else {
			m.State = model.StateResourceList
		}

		return
	}

	if m.State == model.StateResourceList || m.State == model.StateGroupResourceList {
		m.State = model.StateCRDList
		m.SelectedGroup = ""
		m.Filter = ""
		m.FilteredCRDs = m.Crds
		m.FilteredResources = m.Resources

		return
	}

	if m.Filter != "" {
		m.Filter = ""
		m.FilteredCRDs = m.Crds
		m.FilteredResources = m.Resources
		m.CrdCursor = 0
		m.ResourceCursor = 0
	}
}

func (m *Model) moveUp(amount int) {
	switch m.State {
	case model.StateCRDList:
		m.moveUpCRD(amount)
	case model.StateResourceList, model.StateGroupResourceList:
		m.moveUpRes(amount)
	case model.StateYAMLView:
		m.moveUpYAML(amount)
	default:
	}
}

func (m *Model) moveUpCRD(amount int) {
	m.CrdCursor -= amount
	if m.CrdCursor < 0 {
		m.CrdCursor = 0
	}

	if m.CrdCursor < m.CrdScrollOffset {
		m.CrdScrollOffset = m.CrdCursor
	}
}

func (m *Model) moveUpRes(amount int) {
	m.ResourceCursor -= amount
	if m.ResourceCursor < 0 {
		m.ResourceCursor = 0
	}

	if m.ResourceCursor < m.ResScrollOffset {
		m.ResScrollOffset = m.ResourceCursor
	}
}

func (m *Model) moveUpYAML(amount int) {
	m.YamlScrollLine -= amount
	if m.YamlScrollLine < 0 {
		m.YamlScrollLine = 0
	}
}

func (m *Model) moveDown(amount int) {
	switch m.State {
	case model.StateCRDList:
		m.moveDownCRD(amount)
	case model.StateResourceList, model.StateGroupResourceList:
		m.moveDownRes(amount)
	case model.StateYAMLView:
		m.moveDownYAML(amount)
	default:
	}
}

func (m *Model) moveDownYAML(amount int) {
	lines := strings.Split(m.SelectedYAML, "\n")

	m.YamlScrollLine += amount
	if m.YamlScrollLine > len(lines)-1 {
		m.YamlScrollLine = len(lines) - 1
	}

	if m.YamlScrollLine < 0 {
		m.YamlScrollLine = 0
	}
}

func (m *Model) moveDownCRD(amount int) {
	maxHeight := m.Height - 8

	m.CrdCursor += amount
	if m.CrdCursor >= len(m.FilteredCRDs) {
		m.CrdCursor = len(m.FilteredCRDs) - 1
	}

	if m.CrdCursor < 0 {
		m.CrdCursor = 0
	}

	if m.CrdCursor >= m.CrdScrollOffset+maxHeight {
		m.CrdScrollOffset = m.CrdCursor - maxHeight + 1
	}

	if m.CrdScrollOffset < 0 {
		m.CrdScrollOffset = 0
	}
}

func (m *Model) moveDownRes(amount int) {
	maxHeight := m.Height - 8

	m.ResourceCursor += amount
	if m.ResourceCursor >= len(m.FilteredResources) {
		m.ResourceCursor = len(m.FilteredResources) - 1
	}

	if m.ResourceCursor < 0 {
		m.ResourceCursor = 0
	}

	if m.ResourceCursor >= m.ResScrollOffset+maxHeight {
		m.ResScrollOffset = m.ResourceCursor - maxHeight + 1
	}

	if m.ResScrollOffset < 0 {
		m.ResScrollOffset = 0
	}
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.State == model.StateCRDList && len(m.FilteredCRDs) > 0 {
		m.SelectedCRD = m.FilteredCRDs[m.CrdCursor]
		m.SelectedGroup = "" // Clear group if selecting single
		m.State = model.StateResourceList
		m.Loading = true

		return m, m.fetchResources()
	}

	if (m.State == model.StateResourceList || m.State == model.StateGroupResourceList) && len(m.FilteredResources) > 0 {
		m.SelectedRes = m.FilteredResources[m.ResourceCursor]
		m.SelectedCRD = m.SelectedRes.CRD // Sync CRD for fetchYAML
		m.State = model.StateYAMLView
		m.Loading = true

		return m, m.fetchYAML()
	}

	return m, nil
}

func (m *Model) applyFilter() {
	switch m.State {
	case model.StateCRDList:
		m.FilteredCRDs = nil

		for _, crd := range m.Crds {
			if strings.Contains(strings.ToLower(crd.Name), strings.ToLower(m.Filter)) {
				m.FilteredCRDs = append(m.FilteredCRDs, crd)
			}
		}

		m.CrdCursor = 0
		m.CrdScrollOffset = 0
	case model.StateResourceList, model.StateGroupResourceList:
		m.FilteredResources = nil

		for _, res := range m.Resources {
			if strings.Contains(strings.ToLower(res.Name), strings.ToLower(m.Filter)) {
				m.FilteredResources = append(m.FilteredResources, res)
			}
		}

		m.ResourceCursor = 0
		m.ResScrollOffset = 0
	default:
	}
}

func (m Model) View() tea.View {
	var s strings.Builder

	m.renderHeader(&s)

	if m.Err != nil {
		nsSnippet := ""
		if m.State == model.StateResourceList || m.State == model.StateGroupResourceList {
			nsSnippet = ", 'n' to toggle namespace"
		}

		msg := fmt.Sprintf("Error: %v\n\nPress 'esc' to go back%s, 'q' to quit.", m.Err, nsSnippet)
		s.WriteString(errorStyle.Render(msg))
	} else if m.Loading {
		s.WriteString(dimStyle.Render("Loading data from cluster..."))
	} else {
		switch m.State {
		case model.StateCRDList:
			m.renderCRDList(&s)
		case model.StateResourceList:
			m.renderResourceList(&s)
		case model.StateGroupResourceList:
			m.renderGroupResourceList(&s)
		case model.StateYAMLView:
			m.renderYAMLView(&s)
		default:
		}
	}

	posIndicator := m.getPositionIndicator()

	s.WriteString("\n" + m.renderStatusBar(posIndicator))

	return tea.NewView(s.String())
}

func (m Model) getPositionIndicator() string {
	var posIndicator string

	switch m.State {
	case model.StateCRDList:
		if len(m.FilteredCRDs) > 0 {
			posIndicator = fmt.Sprintf("[%d/%d]", m.CrdCursor+1, len(m.FilteredCRDs))
		}
	case model.StateResourceList, model.StateGroupResourceList:
		if len(m.FilteredResources) > 0 {
			posIndicator = fmt.Sprintf("[%d/%d]", m.ResourceCursor+1, len(m.FilteredResources))
		}
	case model.StateYAMLView:
		lines := strings.Split(m.SelectedYAML, "\n")
		posIndicator = fmt.Sprintf("[%d/%d]", m.YamlScrollLine+1, len(lines))
	default:
	}

	return posIndicator
}

func (m Model) renderStatusBar(posIndicator string) string {
	var legend string

	var modeIndicator string

	if m.Mode == model.ModeFiltering {
		modeIndicator = filteringModeStyle.Render(" FILTERING ")
		legend = "enter: confirm • esc: stop searching • backspace: delete"
	} else {
		modeIndicator = browsingModeStyle.Render(" BROWSING ")

		switch m.State {
		case model.StateResourceList, model.StateGroupResourceList:
			legend = "↑/↓, j/k, ^d/^u: navigate • enter: view • n: namespace • esc: back • q: quit"
		case model.StateYAMLView:
			legend = "↑/↓, j/k, ^d/^u: scroll • esc: back • q: quit"
		default:
			legend = "/: filter • ↑/↓, j/k, ^d/^u: navigate • enter: select • g: group • esc: back • q: quit"
		}
	}

	return modeIndicator + " " + dimStyle.Render(posIndicator) + " " + statusStyle.Render(legend)
}

func (m Model) renderHeader(s *strings.Builder) {
	header := titleStyle.Render("kubectl-crdlist")

	switch m.State {
	case model.StateCRDList:
		header += " > " + dimStyle.Render("General")
	case model.StateResourceList:
		header += " > " + dimStyle.Render("CRD Resource: ") + m.SelectedCRD.Name
	case model.StateGroupResourceList:
		header += " > " + dimStyle.Render("CRD Group: ") + m.SelectedGroup
	case model.StateYAMLView:
		header += " > " + dimStyle.Render("YAML: ") + m.SelectedRes.Name
	default:
	}

	s.WriteString(header + "\n\n")
}

func (m Model) renderCRDList(s *strings.Builder) {
	cursor := ""
	if m.Mode == model.ModeFiltering {
		cursor = "█"
	}

	s.WriteString(dimStyle.Render("Filter: ") + m.Filter + cursor + "\n\n")
	s.WriteString(headerStyle.Render(fmt.Sprintf("%-40s %-60s", "GROUP", "CRD NAME")))
	s.WriteString("\n")

	maxItems := m.Height - 8
	start := m.CrdScrollOffset
	end := start + maxItems

	if end > len(m.FilteredCRDs) {
		end = len(m.FilteredCRDs)
	}

	for i := start; i < end; i++ {
		crd := m.FilteredCRDs[i]
		line := fmt.Sprintf("%-40s %-60s", crd.Group, crd.Name)

		if i == m.CrdCursor {
			s.WriteString(selectedStyle.Render(line))
		} else {
			s.WriteString(line)
		}

		s.WriteString("\n")
	}
}

func (m Model) renderResourceList(s *strings.Builder) {
	cursor := ""
	if m.Mode == model.ModeFiltering {
		cursor = "█"
	}

	s.WriteString(dimStyle.Render("Filter: ") + m.Filter + cursor + "\n\n")

	nsDisplay := "namespace: " + m.CurrentNamespace
	if m.AllNamespaces {
		nsDisplay = "all namespaces"
	}

	s.WriteString(dimStyle.Render("Viewing "+nsDisplay+" (Press 'n' to toggle)") + "\n\n")
	s.WriteString(headerStyle.Render(fmt.Sprintf("%-40s %-40s", "NAMESPACE", "NAME")))
	s.WriteString("\n")

	maxItems := m.Height - 8
	start := m.ResScrollOffset
	end := start + maxItems

	if end > len(m.FilteredResources) {
		end = len(m.FilteredResources)
	}

	for i := start; i < end; i++ {
		res := m.FilteredResources[i]
		ns := res.Namespace

		if ns == "" {
			ns = "-"
		}

		line := fmt.Sprintf("%-40s %-40s", ns, res.Name)

		if i == m.ResourceCursor {
			s.WriteString(selectedStyle.Render(line))
		} else {
			s.WriteString(line)
		}

		s.WriteString("\n")
	}
}

func (m Model) renderGroupResourceList(s *strings.Builder) {
	cursor := ""
	if m.Mode == model.ModeFiltering {
		cursor = "█"
	}

	s.WriteString(dimStyle.Render("Filter: ") + m.Filter + cursor + "\n\n")

	nsDisplay := "namespace: " + m.CurrentNamespace
	if m.AllNamespaces {
		nsDisplay = "all namespaces"
	}

	s.WriteString(dimStyle.Render("Viewing "+nsDisplay+" (Press 'n' to toggle)") + "\n\n")
	s.WriteString(headerStyle.Render(fmt.Sprintf("%-20s %-40s %-40s", "NAMESPACE", "CRD NAME", "RESOURCE NAME")))
	s.WriteString("\n")

	maxItems := m.Height - 8
	start := m.ResScrollOffset
	end := start + maxItems

	if end > len(m.FilteredResources) {
		end = len(m.FilteredResources)
	}

	for i := start; i < end; i++ {
		res := m.FilteredResources[i]
		ns := res.Namespace

		if ns == "" {
			ns = "-"
		}

		line := fmt.Sprintf("%-20s %-40s %-40s", ns, res.CRD.Name, res.Name)

		if i == m.ResourceCursor {
			s.WriteString(selectedStyle.Render(line))
		} else {
			s.WriteString(line)
		}

		s.WriteString("\n")
	}
}

func (m Model) renderYAMLView(s *strings.Builder) {
	highlighted := m.highlightYAML(m.SelectedYAML)
	lines := strings.Split(highlighted, "\n")

	start := m.YamlScrollLine
	end := start + (m.Height - 8)

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

func (Model) highlightYAML(source string) string {
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
