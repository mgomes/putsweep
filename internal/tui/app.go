package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mgomes/putsweep/internal/config"
	"github.com/mgomes/putsweep/internal/downloader"
)

type ViewState int

const (
	ViewMain ViewState = iota
	ViewSettings
)

type Model struct {
	view    ViewState
	config  *config.Config
	manager *downloader.Manager

	urlInput textinput.Model

	queue           []*downloader.QueueItem
	completed       []*downloader.QueueItem
	currentProgress downloader.ProgressUpdate

	width  int
	height int

	lastError string

	settingsFocus  int
	scheduleChoice int
	dirInput       textinput.Model
}

func New(cfg *config.Config, manager *downloader.Manager) Model {
	urlInput := textinput.New()
	urlInput.Placeholder = "Paste put.io link here..."
	urlInput.Focus()
	urlInput.Width = 50

	dirInput := textinput.New()
	dirInput.SetValue(cfg.DownloadDir)
	dirInput.Width = 50

	scheduleChoice := 0
	switch cfg.ScheduleMode {
	case config.ScheduleAfterMidnight:
		scheduleChoice = 1
	case config.ScheduleBusinessHours:
		scheduleChoice = 2
	}

	return Model{
		view:           ViewMain,
		config:         cfg,
		manager:        manager,
		urlInput:       urlInput,
		queue:          manager.Queue(),
		completed:      manager.Completed(),
		dirInput:       dirInput,
		scheduleChoice: scheduleChoice,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tea.EnableBracketedPaste,
		listenForProgress(m.manager.ProgressChannel()),
		listenForEvents(m.manager.EventChannel()),
		tick(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.view == ViewSettings {
			return m.updateSettings(msg)
		}
		return m.updateMain(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case ProgressMsg:
		m.currentProgress = downloader.ProgressUpdate(msg)
		cmds = append(cmds, listenForProgress(m.manager.ProgressChannel()))

	case EventMsg:
		m.queue = m.manager.Queue()
		m.completed = m.manager.Completed()
		if msg.Type == downloader.EventDownloadFailed {
			m.lastError = msg.Message
		}
		cmds = append(cmds, listenForEvents(m.manager.EventChannel()))

	case TickMsg:
		m.queue = m.manager.Queue()
		m.completed = m.manager.Completed()
		cmds = append(cmds, tick())

	case ErrorMsg:
		m.lastError = msg.Error.Error()

	case AddedToQueueMsg:
		m.urlInput.SetValue("")
		m.queue = m.manager.Queue()
	}

	return m, tea.Batch(cmds...)
}

func (m Model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "s":
		m.view = ViewSettings
		m.dirInput.SetValue(m.config.DownloadDir)
		return m, nil

	case "enter":
		url := strings.TrimSpace(m.urlInput.Value())
		if url != "" {
			err := m.manager.AddToQueue(url)
			if err != nil {
				m.lastError = err.Error()
			} else {
				m.urlInput.SetValue("")
				m.lastError = ""
			}
		}
		return m, nil

	default:
		m.urlInput, cmd = m.urlInput.Update(msg)
	}

	return m, cmd
}

func (m Model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc", "q":
		m.view = ViewMain
		m.urlInput.Focus()
		return m, nil

	case "tab", "down", "j":
		m.settingsFocus = (m.settingsFocus + 1) % 2

	case "shift+tab", "up", "k":
		m.settingsFocus = (m.settingsFocus - 1 + 2) % 2

	case "left", "h":
		if m.settingsFocus == 0 {
			m.scheduleChoice = (m.scheduleChoice - 1 + 3) % 3
			m.applyScheduleChoice()
		}

	case "right", "l":
		if m.settingsFocus == 0 {
			m.scheduleChoice = (m.scheduleChoice + 1) % 3
			m.applyScheduleChoice()
		}

	case "enter":
		if m.settingsFocus == 1 {
			m.config.DownloadDir = m.dirInput.Value()
			m.config.Save()
		}

	default:
		if m.settingsFocus == 1 {
			m.dirInput, cmd = m.dirInput.Update(msg)
		}
	}

	return m, cmd
}

func (m *Model) applyScheduleChoice() {
	var mode config.ScheduleMode
	switch m.scheduleChoice {
	case 0:
		mode = config.ScheduleAnyTime
	case 1:
		mode = config.ScheduleAfterMidnight
	case 2:
		mode = config.ScheduleBusinessHours
	}
	m.manager.SetScheduleMode(mode)
	m.config.Save()
}

func (m Model) View() string {
	if m.view == ViewSettings {
		return m.renderSettings()
	}
	return m.renderMain()
}

func (m Model) renderMain() string {
	var b strings.Builder

	schedStatus := m.manager.Scheduler().StatusText()
	modeName := string(m.config.ScheduleMode)
	modeName = strings.ReplaceAll(modeName, "_", " ")

	header := titleStyle.Render("putsweep") + "  " +
		dimStyle.Render("Mode: "+modeName) + "  " +
		activeStyle.Render(schedStatus)
	b.WriteString(header + "\n\n")

	b.WriteString(headerStyle.Render("Current Download") + "\n")
	b.WriteString(headerStyle.Render(strings.Repeat("─", 40)) + "\n")

	current := m.manager.Current()
	if current != nil {
		b.WriteString(current.Name + "\n")

		var percent float64
		if m.currentProgress.Total > 0 {
			percent = float64(m.currentProgress.Downloaded) / float64(m.currentProgress.Total)
		}

		bar := renderProgressBar(percent, 40)
		percentStr := fmt.Sprintf(" %.0f%%", percent*100)
		sizeStr := fmt.Sprintf(" (%s / %s)",
			formatBytes(m.currentProgress.Downloaded),
			formatBytes(m.currentProgress.Total))

		b.WriteString(bar + percentStr + sizeStr + "\n")

		if m.currentProgress.Speed > 0 {
			speedStr := fmt.Sprintf("Speed: %s/s", formatBytes(uint64(m.currentProgress.Speed)))
			b.WriteString(dimStyle.Render(speedStr) + "\n")
		}
	} else {
		b.WriteString(dimStyle.Render("No active download") + "\n")
	}
	b.WriteString("\n")

	pendingItems := make([]*downloader.QueueItem, 0)
	for _, item := range m.queue {
		if item.Status == downloader.StatusPending {
			pendingItems = append(pendingItems, item)
		}
	}

	b.WriteString(headerStyle.Render(fmt.Sprintf("Queue (%d items)", len(pendingItems))) + "\n")
	b.WriteString(headerStyle.Render(strings.Repeat("─", 40)) + "\n")

	if len(pendingItems) == 0 {
		b.WriteString(dimStyle.Render("Queue is empty") + "\n")
	} else {
		for i, item := range pendingItems {
			if i >= 5 {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  ... and %d more", len(pendingItems)-5)) + "\n")
				break
			}
			line := fmt.Sprintf("%d. %-30s %10s  %s",
				i+1,
				truncate(item.Name, 30),
				formatBytes(item.Size),
				pendingStyle.Render(item.Status.String()))
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n")

	if len(m.completed) > 0 {
		b.WriteString(headerStyle.Render(fmt.Sprintf("Completed (%d items)", len(m.completed))) + "\n")
		b.WriteString(headerStyle.Render(strings.Repeat("─", 40)) + "\n")

		showCount := 3
		if len(m.completed) < showCount {
			showCount = len(m.completed)
		}
		for i := len(m.completed) - showCount; i < len(m.completed); i++ {
			item := m.completed[i]
			line := completedStyle.Render("✓") + " " + item.Name + "  " +
				dimStyle.Render(formatBytes(item.Size))
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	failedItems := make([]*downloader.QueueItem, 0)
	for _, item := range m.queue {
		if item.Status == downloader.StatusFailed {
			failedItems = append(failedItems, item)
		}
	}
	if len(failedItems) > 0 {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Failed (%d items)", len(failedItems))) + "\n")
		for _, item := range failedItems {
			b.WriteString(errorStyle.Render("✗ "+item.Name+": "+item.Error) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(headerStyle.Render("Add file:") + "\n")
	b.WriteString(inputStyle.Render(m.urlInput.View()) + "\n")

	if m.lastError != "" {
		b.WriteString(errorStyle.Render("Error: "+m.lastError) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("s settings  q quit  enter add"))

	return b.String()
}

func (m Model) renderSettings() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Settings") + "\n\n")

	scheduleLabel := "Schedule Mode"
	if m.settingsFocus == 0 {
		scheduleLabel = activeStyle.Render("> " + scheduleLabel)
	} else {
		scheduleLabel = "  " + scheduleLabel
	}
	b.WriteString(scheduleLabel + "\n")

	modes := []string{"Any Time", "After Midnight", "Business Hours"}
	var modeLine strings.Builder
	modeLine.WriteString("  ")
	for i, mode := range modes {
		if i == m.scheduleChoice {
			modeLine.WriteString(activeStyle.Render("[" + mode + "]"))
		} else {
			modeLine.WriteString(dimStyle.Render(" " + mode + " "))
		}
		if i < len(modes)-1 {
			modeLine.WriteString(" ")
		}
	}
	b.WriteString(modeLine.String() + "\n\n")

	dirLabel := "Download Directory"
	if m.settingsFocus == 1 {
		dirLabel = activeStyle.Render("> " + dirLabel)
	} else {
		dirLabel = "  " + dirLabel
	}
	b.WriteString(dirLabel + "\n")

	if m.settingsFocus == 1 {
		b.WriteString("  " + inputStyle.Render(m.dirInput.View()) + "\n")
	} else {
		b.WriteString("  " + dimStyle.Render(m.config.DownloadDir) + "\n")
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("←/→ change mode  tab switch field  esc back  enter save"))

	return b.String()
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s + strings.Repeat(" ", max-len(s))
	}
	return s[:max-3] + "..."
}

type AuthModel struct {
	tokenInput textinput.Model
	error      string
	width      int
	height     int
}

func NewAuthModel() AuthModel {
	ti := textinput.New()
	ti.Placeholder = "Paste your OAuth token here..."
	ti.Focus()
	ti.Width = 60
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'

	return AuthModel{
		tokenInput: ti,
	}
}

func (m AuthModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m AuthModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			token := strings.TrimSpace(m.tokenInput.Value())
			if token != "" {
				return m, func() tea.Msg {
					return TokenSubmitMsg{Token: token}
				}
			}
		}
		m.tokenInput, cmd = m.tokenInput.Update(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case AuthErrorMsg:
		m.error = msg.Error

	default:
		m.tokenInput, cmd = m.tokenInput.Update(msg)
	}

	return m, cmd
}

func (m AuthModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("putsweep - Setup") + "\n\n")
	b.WriteString("To get started, you need a Put.io OAuth token.\n\n")
	b.WriteString("1. Go to " + activeStyle.Render("https://app.put.io/settings/account") + "\n")
	b.WriteString("2. Scroll down to 'OAuth Token'\n")
	b.WriteString("3. Click 'Create a new OAuth token'\n")
	b.WriteString("4. Copy the token and paste it below\n\n")

	b.WriteString(headerStyle.Render("OAuth Token:") + "\n")

	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1)

	b.WriteString(style.Render(m.tokenInput.View()) + "\n")

	if m.error != "" {
		b.WriteString("\n" + errorStyle.Render("Error: "+m.error) + "\n")
	}

	b.WriteString("\n" + helpStyle.Render("enter submit  ctrl+c quit"))

	return b.String()
}

type TokenSubmitMsg struct {
	Token string
}

type AuthErrorMsg struct {
	Error string
}

type AuthSuccessMsg struct {
	Token string
}
