package main

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/rorycaraher/abletonctl/internal/configcheck"
	"github.com/rorycaraher/abletonctl/internal/demolink"
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
	linkedStyle    = lipgloss.NewStyle().Width(9).Foreground(lipgloss.Color("2"))
	ambiguousStyle = lipgloss.NewStyle().Width(9).Foreground(lipgloss.Color("3"))
	danglingStyle  = lipgloss.NewStyle().Width(9).Bold(true).Foreground(lipgloss.Color("1"))
	unlinkedStyle  = lipgloss.NewStyle().Width(9).Faint(true)
)

func statusLabel(s demolink.Status) string {
	switch s {
	case demolink.Linked:
		return linkedStyle.Render(string(s))
	case demolink.Ambiguous:
		return ambiguousStyle.Render(string(s))
	case demolink.Dangling:
		return danglingStyle.Render(string(s))
	default:
		return unlinkedStyle.Render(string(s))
	}
}

var (
	localStyle  = lipgloss.NewStyle().Width(6)
	remoteStyle = lipgloss.NewStyle().Width(6).Faint(true)
	bothStyle   = lipgloss.NewStyle().Width(6).Foreground(lipgloss.Color("6"))
)

func locationLabel(l demolink.Location) string {
	switch l {
	case demolink.Both:
		return bothStyle.Render(string(l))
	case demolink.Remote:
		return remoteStyle.Render(string(l))
	default:
		return localStyle.Render(string(l))
	}
}
