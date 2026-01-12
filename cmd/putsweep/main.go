package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mgomes/putsweep/internal/auth"
	"github.com/mgomes/putsweep/internal/config"
	"github.com/mgomes/putsweep/internal/downloader"
	"github.com/mgomes/putsweep/internal/putio"
	"github.com/mgomes/putsweep/internal/tui"
)

func main() {
	destDir := flag.String("dir", "", "download destination directory")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *destDir != "" {
		cfg.DownloadDir = *destDir
	}

	if cfg.OAuthToken == "" {
		token, err := runAuthFlow()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}
		cfg.OAuthToken = token
		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
			os.Exit(1)
		}
	}

	valid, err := auth.ValidateToken(cfg.OAuthToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to validate token: %v\n", err)
		os.Exit(1)
	}
	if !valid {
		fmt.Fprintln(os.Stderr, "OAuth token is invalid or expired. Please re-authenticate.")
		cfg.OAuthToken = ""
		cfg.Save()

		token, err := runAuthFlow()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}
		cfg.OAuthToken = token
		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
			os.Exit(1)
		}
	}

	putioClient := putio.NewClient(auth.NewClient(cfg.OAuthToken))

	manager := downloader.NewManager(cfg, putioClient)
	manager.Start()
	defer manager.Stop()

	model := tui.New(cfg, manager)
	program := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runAuthFlow() (string, error) {
	model := newAuthRunner()
	program := tea.NewProgram(model)

	finalModel, err := program.Run()
	if err != nil {
		return "", err
	}

	if runner, ok := finalModel.(authRunner); ok {
		if runner.token != "" {
			return runner.token, nil
		}
	}

	return "", fmt.Errorf("authentication cancelled")
}

type authRunner struct {
	authModel tui.AuthModel
	token     string
	error     string
}

func newAuthRunner() authRunner {
	return authRunner{
		authModel: tui.NewAuthModel(),
	}
}

func (m authRunner) Init() tea.Cmd {
	return tea.Batch(m.authModel.Init(), tea.EnableBracketedPaste)
}

func (m authRunner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tui.TokenSubmitMsg:
		valid, err := auth.ValidateToken(msg.Token)
		if err != nil {
			m.authModel = updateAuthError(m.authModel, "Failed to validate token: "+err.Error())
			return m, nil
		}
		if !valid {
			m.authModel = updateAuthError(m.authModel, "Invalid token. Please check and try again.")
			return m, nil
		}
		m.token = msg.Token
		return m, tea.Quit

	default:
		newModel, cmd := m.authModel.Update(msg)
		if am, ok := newModel.(tui.AuthModel); ok {
			m.authModel = am
		}
		return m, cmd
	}
}

func (m authRunner) View() string {
	return m.authModel.View()
}

func updateAuthError(m tui.AuthModel, errMsg string) tui.AuthModel {
	newModel, _ := m.Update(tui.AuthErrorMsg{Error: errMsg})
	if am, ok := newModel.(tui.AuthModel); ok {
		return am
	}
	return m
}
