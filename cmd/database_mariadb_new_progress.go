package cmd

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/babbage88/infra-cli/tui"
	coredeploy "github.com/babbage88/infra-core/deployment"
)

type mariaDBInstallLogEntry = coredeploy.MariaDBInstallLogEntry
type mariaDBInstallLogKind = coredeploy.MariaDBInstallLogKind
type mariaDBInstallLogSink = coredeploy.MariaDBInstallLogSink

const (
	mariaDBInstallLogStatus  = coredeploy.MariaDBInstallLogStatus
	mariaDBInstallLogCommand = coredeploy.MariaDBInstallLogCommand
	mariaDBInstallLogStdout  = coredeploy.MariaDBInstallLogStdout
	mariaDBInstallLogStderr  = coredeploy.MariaDBInstallLogStderr
)

type mariaDBInstallLogMsg mariaDBInstallLogEntry

type mariaDBInstallDoneMsg struct {
	result coredeploy.MariaDBInstallResult
	err    error
}

type mariaDBInstallViewportModel struct {
	viewport     viewport.Model
	entries      []mariaDBInstallLogEntry
	statuses     []mariaDBInstallLogEntry
	done         bool
	err          error
	result       coredeploy.MariaDBInstallResult
	contentWidth int
}

func runMariaDBInstall(req coredeploy.MariaDBInstallRequest) (coredeploy.MariaDBInstallResult, error) {
	if !tui.IsInteractive() {
		return coredeploy.InstallMariaDB(req)
	}

	model := newMariaDBInstallViewportModel()
	program := tea.NewProgram(model)
	go func() {
		logger := func(entry mariaDBInstallLogEntry) {
			program.Send(mariaDBInstallLogMsg(entry))
		}
		result, err := coredeploy.InstallMariaDBWithLog(req, logger)
		program.Send(mariaDBInstallDoneMsg{result: result, err: err})
	}()

	result, err := program.Run()
	if err != nil {
		return coredeploy.MariaDBInstallResult{}, fmt.Errorf("run MariaDB install output UI: %w", err)
	}
	finalModel, ok := result.(mariaDBInstallViewportModel)
	if !ok {
		return coredeploy.MariaDBInstallResult{}, fmt.Errorf("run MariaDB install output UI: unexpected model %T", result)
	}
	return finalModel.result, finalModel.err
}

func newMariaDBInstallViewportModel() mariaDBInstallViewportModel {
	vp := viewport.New(viewport.WithWidth(104), viewport.WithHeight(16))
	vp.SoftWrap = false
	vp.FillHeight = true
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#18D7FF")).
		Padding(0, 1)

	model := mariaDBInstallViewportModel{viewport: vp, contentWidth: 100}
	model.appendEntry(mariaDBInstallLogEntry{Kind: mariaDBInstallLogStatus, Body: "Preparing MariaDB install..."})
	return model
}

func (m mariaDBInstallViewportModel) Init() tea.Cmd {
	return nil
}

func (m mariaDBInstallViewportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width := min(112, max(72, msg.Width-8))
		height := min(18, max(8, msg.Height-13))
		m.viewport.SetWidth(width)
		m.viewport.SetHeight(height)
		m.contentWidth = max(32, width-m.viewport.Style.GetHorizontalFrameSize()-2)
		m.renderContent()
	case tea.KeyPressMsg:
		if m.done {
			switch msg.String() {
			case "enter", "q", "esc", "ctrl+c":
				return m, tea.Quit
			}
		}
	case mariaDBInstallLogMsg:
		m.appendEntry(mariaDBInstallLogEntry(msg))
	case mariaDBInstallDoneMsg:
		m.done = true
		m.result = msg.result
		m.err = msg.err
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m mariaDBInstallViewportModel) View() tea.View {
	status := mariaDBInstallMutedStyle.Render("running")
	if m.done {
		status = mariaDBInstallSuccessStyle.Render("done")
		if m.err != nil {
			status = mariaDBInstallErrorStyle.Render("failed")
		}
	}

	header := mariaDBInstallTitleStyle.Render("MariaDB deploy") + " " + status + "\n"
	statusPanel := m.statusPanel()
	outputTitle := mariaDBInstallSectionTitleStyle.Render("command / stdout / stderr")
	footer := mariaDBInstallHelpStyle.Render("Scroll: up/down, pgup/pgdn")

	donePrompt := ""
	if m.done {
		doneText := "MariaDB deploy completed successfully. Press enter, q, or esc to continue."
		doneStyle := mariaDBInstallDonePromptStyle
		if m.err != nil {
			doneText = "MariaDB deploy failed: " + mariaDBInstallPromptError(m.err, m.viewport.Width()) + " Press enter, q, or esc to continue."
			doneStyle = mariaDBInstallErrorPromptStyle
		}
		donePrompt = "\n" + doneStyle.Render(doneText)
	}

	return tea.NewView(header + statusPanel + "\n" + outputTitle + "\n" + m.viewport.View() + "\n" + footer + donePrompt)
}

func (m *mariaDBInstallViewportModel) appendEntry(entry mariaDBInstallLogEntry) {
	entry.Label = tui.CleanTerminalLogText(entry.Label)
	entry.Body = strings.TrimRight(tui.CleanTerminalLogText(entry.Body), "\n")
	if entry.Body == "" && entry.Kind != mariaDBInstallLogCommand {
		return
	}
	m.entries = append(m.entries, entry)
	if entry.Kind == mariaDBInstallLogStatus {
		m.statuses = append(m.statuses, entry)
	}
	m.renderContent()
}

func (m *mariaDBInstallViewportModel) renderContent() {
	m.viewport.SetContent(strings.TrimRight(renderMariaDBInstallEntries(m.entries, m.contentWidth), "\n"))
	m.viewport.GotoBottom()
}

func (m mariaDBInstallViewportModel) statusPanel() string {
	width := max(32, m.viewport.Width())
	style := mariaDBInstallStatusPanelStyle.Width(max(20, width-mariaDBInstallStatusPanelStyle.GetHorizontalFrameSize()))
	bodyWidth := max(20, width-style.GetHorizontalFrameSize())
	statuses := m.statuses
	if len(statuses) > 4 {
		statuses = statuses[len(statuses)-4:]
	}

	var lines []string
	for _, status := range statuses {
		line := strings.TrimSpace(status.Body)
		if line == "" {
			continue
		}
		lines = append(lines, mariaDBInstallStatusStyle.Render(truncatePlain(line, bodyWidth)))
	}
	if len(lines) == 0 {
		lines = append(lines, mariaDBInstallStatusStyle.Render("waiting for status..."))
	}

	return mariaDBInstallSectionTitleStyle.Render("status") + "\n" +
		style.Render(strings.Join(lines, "\n")) + "\n"
}

func renderMariaDBInstallEntries(entries []mariaDBInstallLogEntry, width int) string {
	var builder strings.Builder
	for _, entry := range entries {
		if entry.Kind == mariaDBInstallLogStatus {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(renderMariaDBInstallEntry(entry, width))
	}
	if builder.Len() == 0 {
		builder.WriteString(mariaDBInstallMutedStyle.Render("Waiting for command output..."))
		builder.WriteString("\n")
	}
	return builder.String()
}

func renderMariaDBInstallEntry(entry mariaDBInstallLogEntry, width int) string {
	width = max(32, width)
	bodyWidth := max(20, width-2)

	headerStyle := mariaDBInstallCommandHeaderStyle
	bodyStyle := mariaDBInstallCommandBlockStyle
	label := strings.TrimSpace(entry.Label)
	if label == "" {
		label = "command"
	}

	switch entry.Kind {
	case mariaDBInstallLogCommand:
		label = "command: " + label
		entry.Body = "$ " + entry.Body
	case mariaDBInstallLogStdout:
		label = "stdout: " + label
	case mariaDBInstallLogStderr:
		label = "stderr: " + label
		headerStyle = mariaDBInstallErrorHeaderStyle
		bodyStyle = mariaDBInstallErrorBlockStyle
	}

	label = truncatePlain(label, bodyWidth)
	var builder strings.Builder
	builder.WriteString(headerStyle.Render(label))
	builder.WriteString("\n")
	for _, line := range strings.Split(wrapPreformatted(entry.Body, bodyWidth), "\n") {
		builder.WriteString(bodyStyle.Render("  " + line))
		builder.WriteString("\n")
	}
	return builder.String()
}

func mariaDBInstallPromptError(err error, width int) string {
	if err == nil {
		return ""
	}
	value := tui.CleanTerminalLogText(err.Error())
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "unknown error."
	}

	const suffix = " See output above."
	maxWidth := max(48, width-len("MariaDB deploy failed:  Press enter, q, or esc to continue."))
	if len(value)+len(suffix) > maxWidth {
		value = truncatePlain(value, max(16, maxWidth-len(suffix)))
	}
	return value + suffix
}

var (
	mariaDBInstallSectionTitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#68717C")).PaddingLeft(1)
	mariaDBInstallTitleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8A929E"))
	mariaDBInstallMutedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#5D646F"))
	mariaDBInstallHelpStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F2F6FF"))
	mariaDBInstallSuccessStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#6F9276"))
	mariaDBInstallErrorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#AA7070"))
	mariaDBInstallStatusStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C747D"))
	mariaDBInstallCommandBlockStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#707883"))
	mariaDBInstallCommandHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#848B96"))
	mariaDBInstallErrorHeaderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D38B8B"))
	mariaDBInstallErrorBlockStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#C88989"))
	mariaDBInstallStatusPanelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#18D7FF")).Padding(0, 1)
	mariaDBInstallDonePromptStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B8FFE8"))
	mariaDBInstallErrorPromptStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFC2C2"))
)
