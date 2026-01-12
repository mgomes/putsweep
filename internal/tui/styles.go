package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("241"))

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	completedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	progressBarWidth  = 40
	progressFillChar  = "█"
	progressEmptyChar = "░"

	progressFillStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42"))

	progressEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

func renderProgressBar(percent float64, width int) string {
	if width <= 0 {
		width = progressBarWidth
	}

	filled := int(percent * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	bar := progressFillStyle.Render(repeat(progressFillChar, filled)) +
		progressEmptyStyle.Render(repeat(progressEmptyChar, empty))

	return bar
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
