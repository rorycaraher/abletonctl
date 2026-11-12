package main

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/rorycaraher/abletonctl/internal/configcheck"
	"github.com/rorycaraher/abletonctl/internal/demolist"
)

// Styles degrade to plain text automatically when stdout isn't a terminal
// or NO_COLOR is set, so piped output stays unstyled.
var (
	okStyle   = lipgloss.NewStyle().Width(4).Foreground(lipgloss.Color("2"))
	warnStyle = lipgloss.NewStyle().Width(4).Foreground(lipgloss.Color("3"))
	failStyle = lipgloss.NewStyle().Width(4).Bold(true).Foreground(lipgloss.Color("1"))
	dimStyle  = lipgloss.NewStyle().Faint(true)
	boldStyle = lipgloss.NewStyle().Bold(true)
)

func levelLabel(l configcheck.Level) string {
	switch l {
	case configcheck.OK:
		return okStyle.Render(l.String())
	case configcheck.Warn:
		return warnStyle.Render(l.String())
	default:
		return failStyle.Render(l.String())
	}
}

var (
	localStyle  = lipgloss.NewStyle().Width(6)
	remoteStyle = lipgloss.NewStyle().Width(6).Faint(true)
	bothStyle   = lipgloss.NewStyle().Width(6).Foreground(lipgloss.Color("6"))
)

func locationLabel(l demolist.Location) string {
	switch l {
	case demolist.Both:
		return bothStyle.Render(string(l))
	case demolist.Remote:
		return remoteStyle.Render(string(l))
	default:
		return localStyle.Render(string(l))
	}
}
