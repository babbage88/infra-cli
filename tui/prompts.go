package tui

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	promptLabelStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E6F4F1"))
	promptExampleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#C9D7E3"))
	promptHintStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#C9D7E3"))
	promptErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#A33131"))
	promptHelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#B9C7D4"))
)

// CleanTerminalLogText removes terminal control bytes from command output before
// it is rendered inside styled Bubble Tea views.
func CleanTerminalLogText(value string) string {
	value = stripANSISequences(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")

	runes := make([]rune, 0, len(value))
	lineStart := 0
	for _, r := range value {
		switch {
		case r == '\n':
			runes = append(runes, r)
			lineStart = len(runes)
		case r == '\r':
			runes = runes[:lineStart]
		case r == '\t':
			runes = append(runes, r)
		case r == '\b':
			if len(runes) > lineStart {
				runes = runes[:len(runes)-1]
			}
		case r >= 0x20 && r != 0x7f:
			runes = append(runes, r)
		}
	}

	lines := strings.Split(string(runes), "\n")
	cleaned := lines[:0]
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			if len(cleaned) == 0 || cleaned[len(cleaned)-1] == "" {
				continue
			}
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimRight(strings.Join(cleaned, "\n"), "\n")
}

func stripANSISequences(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if value[i] != 0x1b {
			builder.WriteByte(value[i])
			continue
		}
		i++
		if i >= len(value) {
			break
		}
		switch value[i] {
		case '[':
			for i+1 < len(value) {
				i++
				if value[i] >= 0x40 && value[i] <= 0x7e {
					break
				}
			}
		case ']':
			for i+1 < len(value) {
				i++
				if value[i] == 0x07 {
					break
				}
				if value[i] == 0x1b && i+1 < len(value) && value[i+1] == '\\' {
					i++
					break
				}
			}
		default:
			// Two-byte escape sequences, such as ESC c.
		}
	}
	return builder.String()
}

type inputModel struct {
	label        string
	example      string
	defaultValue string
	required     bool
	password     bool
	input        textinput.Model
	err          string
	done         bool
	cancel       bool
}

func newInputModel(label, example, defaultValue string, required bool, password bool) inputModel {
	input := textinput.New()
	input.Prompt = "> "
	input.Placeholder = defaultValue
	input.SetValue("")
	input.SetWidth(72)
	if password {
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '*'
	}
	input.Focus()

	return inputModel{
		label:        label,
		example:      example,
		defaultValue: defaultValue,
		required:     required,
		password:     password,
		input:        input,
	}
}

func (m inputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancel = true
			return m, tea.Quit
		case "enter":
			value := strings.TrimSpace(m.input.Value())
			if value == "" && m.defaultValue != "" {
				m.done = true
				return m, tea.Quit
			}
			if value == "" && m.required {
				m.err = "Please enter a value."
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m inputModel) View() tea.View {
	var builder strings.Builder
	builder.WriteString(renderPromptLabel(m.label, m.example))
	if m.defaultValue != "" {
		hint := m.defaultValue
		if m.password {
			hint = "press enter to use current default"
		}
		builder.WriteString(" " + promptHintStyle.Render("["+hint+"]"))
	}
	builder.WriteString("\n")
	builder.WriteString(m.input.View())
	if m.err != "" {
		builder.WriteString("\n" + promptErrorStyle.Render(m.err))
	}
	builder.WriteString("\n")

	view := tea.NewView(builder.String())
	view.Cursor = m.input.Cursor()
	return view
}

type confirmModel struct {
	label      string
	defaultYes bool
	choice     *bool
	cancel     bool
	err        string
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch strings.ToLower(msg.String()) {
		case "ctrl+c", "esc":
			m.cancel = true
			return m, tea.Quit
		case "enter":
			choice := m.defaultYes
			m.choice = &choice
			return m, tea.Quit
		case "y":
			choice := true
			m.choice = &choice
			return m, tea.Quit
		case "n":
			choice := false
			m.choice = &choice
			return m, tea.Quit
		default:
			m.err = "Press y or n."
		}
	}
	return m, nil
}

func (m confirmModel) View() tea.View {
	defaultLabel := "y/N"
	if m.defaultYes {
		defaultLabel = "Y/n"
	}

	var builder strings.Builder
	builder.WriteString(promptLabelStyle.Render(m.label))
	builder.WriteString(" " + promptHintStyle.Render("["+defaultLabel+"]"))
	builder.WriteString("\n")
	builder.WriteString(promptHelpStyle.Render("y yes  n no  enter default"))
	if m.err != "" {
		builder.WriteString("\n" + promptErrorStyle.Render(m.err))
	}
	builder.WriteString("\n")
	return tea.NewView(builder.String())
}

type textareaModel struct {
	label  string
	input  textarea.Model
	cancel bool
}

func newTextareaModel(label, defaultValue string) textareaModel {
	input := textarea.New()
	input.Prompt = "  "
	input.ShowLineNumbers = true
	input.SetWidth(88)
	input.SetHeight(14)
	input.SetValue(defaultValue)
	input.Focus()

	return textareaModel{
		label: label,
		input: input,
	}
}

func (m textareaModel) Init() tea.Cmd {
	return m.input.Focus()
}

func (m textareaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.input.SetWidth(min(100, max(48, msg.Width-6)))
		m.input.SetHeight(min(18, max(8, msg.Height-8)))
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancel = true
			return m, tea.Quit
		case "ctrl+s":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m textareaModel) View() tea.View {
	var builder strings.Builder
	builder.WriteString(promptLabelStyle.Render(m.label))
	builder.WriteString("\n")
	builder.WriteString(m.input.View())
	builder.WriteString("\n")
	builder.WriteString(promptHelpStyle.Render("ctrl+s save  esc cancel"))
	builder.WriteString("\n")

	view := tea.NewView(builder.String())
	view.Cursor = m.input.Cursor()
	return view
}

type selectItem string

func (i selectItem) FilterValue() string { return string(i) }
func (i selectItem) Title() string       { return string(i) }
func (i selectItem) Description() string { return "" }

type selectModel struct {
	label  string
	list   list.Model
	value  string
	cancel bool
}

func newSelectModel(label string, options []string, defaultValue string) selectModel {
	items := make([]list.Item, 0, len(options))
	defaultIndex := 0
	for i, option := range options {
		items = append(items, selectItem(option))
		if defaultValue != "" && option == defaultValue {
			defaultIndex = i
		}
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("#2F6F73")).BorderLeftForeground(lipgloss.Color("#2F6F73"))

	listModel := list.New(items, delegate, 80, min(14, len(options)+5))
	listModel.Title = label
	listModel.SetShowStatusBar(false)
	listModel.SetShowPagination(false)
	listModel.SetShowHelp(false)
	listModel.SetFilteringEnabled(false)
	listModel.Select(defaultIndex)

	return selectModel{label: label, list: listModel}
}

func (m selectModel) Init() tea.Cmd {
	return nil
}

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(min(14, msg.Height))
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.cancel = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(selectItem); ok {
				m.value = string(item)
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectModel) View() tea.View {
	view := strings.TrimRight(m.list.View(), "\n")
	footer := m.paginationFooter()
	if footer != "" {
		view += "\n" + footer
	}
	return tea.NewView(view)
}

func (m selectModel) paginationFooter() string {
	total := len(m.list.VisibleItems())
	if total == 0 {
		return promptHelpStyle.Render("No options available.")
	}

	page := m.list.Paginator.Page
	perPage := m.list.Paginator.PerPage
	if perPage <= 0 {
		perPage = total
	}

	start := page*perPage + 1
	if start > total {
		start = total
	}
	end := min(page*perPage+perPage, total)

	parts := []string{fmt.Sprintf("Showing %d-%d of %d.", start, end, total)}
	if page > 0 {
		parts = append(parts, "more above")
	}
	if end < total {
		parts = append(parts, "more below")
	}
	parts = append(parts, "move: up/down or j/k", "page: pgup/pgdn", "select: enter", "quit: esc/q")

	return promptHelpStyle.Render(strings.Join(parts, "  "))
}

// IsInteractive reports whether stdin is attached to a terminal.
func IsInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// Input prompts until a non-empty value is provided, unless a default is available.
func Input(label, defaultValue string) string {
	if IsInteractive() {
		if value, ok := runInput(label, "", defaultValue, true, false); ok {
			return value
		}
	}
	return inputPlain(label, defaultValue, true)
}

// InputWithExample is Input with an inline example in the label.
func InputWithExample(label, example, defaultValue string) string {
	if IsInteractive() {
		if value, ok := runInput(label, example, defaultValue, true, false); ok {
			return value
		}
	}
	return inputPlain(labelWithExample(label, example), defaultValue, true)
}

// OptionalInput prompts once and permits an empty value.
func OptionalInput(label, defaultValue string) string {
	if IsInteractive() {
		if value, ok := runInput(label, "", defaultValue, false, false); ok {
			return value
		}
	}
	return inputPlain(label, defaultValue, false)
}

// Password prompts for masked input until a value is provided, unless a default is available.
func Password(label, defaultValue string) string {
	if IsInteractive() {
		if value, ok := runInput(label, "", defaultValue, true, true); ok {
			return value
		}
	}
	return inputPlain(label, defaultValue, true)
}

// PasswordWithExample is Password with an inline example in the label.
func PasswordWithExample(label, example, defaultValue string) string {
	if IsInteractive() {
		if value, ok := runInput(label, example, defaultValue, true, true); ok {
			return value
		}
	}
	return inputPlain(labelWithExample(label, example), defaultValue, true)
}

// TextArea prompts for a multi-line value.
func TextArea(label, defaultValue string) string {
	if IsInteractive() {
		if value, ok := runTextArea(label, defaultValue); ok {
			return value
		}
	}
	return textAreaPlain(label, defaultValue)
}

// YesNo prompts for a boolean value.
func YesNo(label string, defaultYes bool) bool {
	if IsInteractive() {
		model := confirmModel{label: label, defaultYes: defaultYes}
		result, err := tea.NewProgram(model).Run()
		if err == nil {
			if selected, ok := result.(confirmModel); ok && !selected.cancel && selected.choice != nil {
				return *selected.choice
			}
		}
	}
	return yesNoPlain(label, defaultYes)
}

// SelectOption prompts for one value from options.
func SelectOption(label string, options []string, defaultValue string) string {
	if len(options) == 0 {
		if defaultValue != "" {
			return defaultValue
		}
		slog.Error("SelectOption called with no options")
		os.Exit(1)
	}

	if IsInteractive() {
		model := newSelectModel(label, options, defaultValue)
		result, err := tea.NewProgram(model).Run()
		if err == nil {
			if selected, ok := result.(selectModel); ok && !selected.cancel && selected.value != "" {
				return selected.value
			}
		}
	}

	return selectOptionPlain(label, options, defaultValue)
}

func runInput(label, example, defaultValue string, required bool, password bool) (string, bool) {
	model := newInputModel(label, example, defaultValue, required, password)
	result, err := tea.NewProgram(model).Run()
	if err != nil {
		return "", false
	}

	finalModel, ok := result.(inputModel)
	if !ok || finalModel.cancel {
		os.Exit(1)
	}

	value := strings.TrimSpace(finalModel.input.Value())
	if value == "" {
		return defaultValue, true
	}
	return value, true
}

func runTextArea(label, defaultValue string) (string, bool) {
	model := newTextareaModel(label, defaultValue)
	result, err := tea.NewProgram(model).Run()
	if err != nil {
		return "", false
	}

	finalModel, ok := result.(textareaModel)
	if !ok || finalModel.cancel {
		os.Exit(1)
	}

	return finalModel.input.Value(), true
}

func inputPlain(label, defaultValue string, required bool) string {
	reader := bufio.NewReader(os.Stdin)

	for {
		if defaultValue != "" {
			fmt.Printf("%s [%s]: ", label, defaultValue)
		} else {
			fmt.Printf("%s: ", label)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("Failed to read input", "error", err.Error())
			os.Exit(1)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			if defaultValue != "" || !required {
				return defaultValue
			}
			continue
		}

		return input
	}
}

func textAreaPlain(label, defaultValue string) string {
	fmt.Println(label)
	fmt.Println("Enter script content, then send EOF when done.")
	if defaultValue != "" {
		fmt.Println(defaultValue)
	}

	scanner := bufio.NewScanner(os.Stdin)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		slog.Error("Failed to read input", "error", err.Error())
		os.Exit(1)
	}
	if len(lines) == 0 {
		return defaultValue
	}
	return strings.Join(lines, "\n")
}

func yesNoPlain(label string, defaultYes bool) bool {
	reader := bufio.NewReader(os.Stdin)
	defaultLabel := "y/N"
	if defaultYes {
		defaultLabel = "Y/n"
	}

	for {
		fmt.Printf("%s [%s]: ", label, defaultLabel)
		input, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("Failed to read input", "error", err.Error())
			os.Exit(1)
		}

		switch strings.ToLower(strings.TrimSpace(input)) {
		case "":
			return defaultYes
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
	}
}

func selectOptionPlain(label string, options []string, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	defaultIndex := -1
	for i, option := range options {
		fmt.Printf("%d. %s\n", i+1, option)
		if defaultValue != "" && option == defaultValue {
			defaultIndex = i
		}
	}

	for {
		switch {
		case defaultIndex >= 0:
			fmt.Printf("%s [%d]: ", label, defaultIndex+1)
		case defaultValue != "":
			fmt.Printf("%s [%s]: ", label, defaultValue)
		default:
			fmt.Printf("%s: ", label)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("Failed to read input", "error", err.Error())
			os.Exit(1)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			if defaultIndex >= 0 {
				return options[defaultIndex]
			}
			if defaultValue != "" {
				return defaultValue
			}
			continue
		}

		selection, err := strconv.Atoi(input)
		if err == nil && selection >= 1 && selection <= len(options) {
			return options[selection-1]
		}

		for _, option := range options {
			if input == option {
				return option
			}
		}

		fmt.Printf("Please enter a number between 1 and %d or a listed value.\n", len(options))
	}
}

func labelWithExample(label, example string) string {
	if strings.TrimSpace(example) == "" {
		return label
	}
	return fmt.Sprintf("%s (example: %s)", label, example)
}

func renderPromptLabel(label, example string) string {
	rendered := promptLabelStyle.Render(label)
	if strings.TrimSpace(example) == "" {
		return rendered
	}
	return rendered + " " + promptExampleStyle.Render("(example: "+example+")")
}
