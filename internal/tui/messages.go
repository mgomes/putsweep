package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mgomes/putsweep/internal/downloader"
)

type ProgressMsg downloader.ProgressUpdate

type EventMsg downloader.Event

type TickMsg time.Time

type ErrorMsg struct {
	Error error
}

type AddedToQueueMsg struct{}

func listenForProgress(ch <-chan downloader.ProgressUpdate) tea.Cmd {
	return func() tea.Msg {
		update := <-ch
		return ProgressMsg(update)
	}
}

func listenForEvents(ch <-chan downloader.Event) tea.Cmd {
	return func() tea.Msg {
		event := <-ch
		return EventMsg(event)
	}
}

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}
