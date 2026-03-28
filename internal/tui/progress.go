// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/blubskye/discord2stoat/internal/pipeline"
)

var (
	progressBarFull  = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render("█")
	progressBarEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render("░")
	doneStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓")
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	inProgressStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("…")
)

// channelState tracks live progress for one channel.
type channelState struct {
	name       string
	fetchCount int
	fetchTotal int
	postCount  map[string]int
	postTotal  map[string]int
	done       bool
	err        error
}

// ProgressModel is the bubbletea model for Screen 3.
type ProgressModel struct {
	channels     map[string]*channelState
	channelOrder []string
	categories   []categoryProgress

	rolesCreated int
	rolesTotal   int
	structDone   map[string]bool

	targets     []string
	paused      bool
	cancelled   bool
	pipelineErr error

	progressCh <-chan pipeline.ProgressEvent
}

type categoryProgress struct {
	name       string
	channelIDs []string
}

// NewProgressModel creates Screen 3.
func NewProgressModel(
	progressCh <-chan pipeline.ProgressEvent,
	targets []string,
	categories []categoryProgress,
	channels []*struct{ ID, Name string },
) ProgressModel {
	m := ProgressModel{
		channels:   make(map[string]*channelState),
		targets:    targets,
		structDone: make(map[string]bool),
		categories: categories,
		progressCh: progressCh,
	}
	for _, ch := range channels {
		m.channels[ch.ID] = &channelState{
			name:      ch.Name,
			postCount: make(map[string]int),
			postTotal: make(map[string]int),
		}
		m.channelOrder = append(m.channelOrder, ch.ID)
	}
	return m
}

// WaitForEvent is a bubbletea Cmd that reads the next ProgressEvent.
// When the channel is closed it returns msgPipelineDone to stop the event loop.
func (m ProgressModel) WaitForEvent() tea.Cmd {
	return func() tea.Msg {
		e, ok := <-m.progressCh
		if !ok {
			return msgPipelineDone{}
		}
		return e
	}
}

func (m ProgressModel) Init() tea.Cmd {
	return m.WaitForEvent()
}

func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pipeline.ProgressEvent:
		m.applyEvent(msg)
		return m, m.WaitForEvent()

	case msgPipelineDone:
		// Pipeline finished; stop listening for events.
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "p":
			m.paused = !m.paused
		case "c", "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *ProgressModel) applyEvent(e pipeline.ProgressEvent) {
	switch e.Kind {
	case pipeline.EventRoleCreated:
		m.rolesCreated++
		m.rolesTotal = e.RolesTotal
	case pipeline.EventStructureDone:
		m.structDone[e.TargetName] = true
	case pipeline.EventChannelFetch:
		if s, ok := m.channels[e.ChannelID]; ok {
			s.fetchCount = e.Count
			s.fetchTotal = e.Total
		}
	case pipeline.EventChannelPost:
		if s, ok := m.channels[e.ChannelID]; ok {
			s.postCount[e.TargetName] = e.Count
			s.postTotal[e.TargetName] = e.Total
		}
	case pipeline.EventChannelDone:
		if s, ok := m.channels[e.ChannelID]; ok {
			s.done = true
		}
	case pipeline.EventChannelError:
		if s, ok := m.channels[e.ChannelID]; ok {
			s.err = e.Err
		}
	case pipeline.EventPipelineError:
		m.pipelineErr = e.Err
	}
}

func (m ProgressModel) View() string {
	var sb strings.Builder

	if m.rolesTotal > 0 {
		sb.WriteString(renderBar("Roles", m.rolesCreated, m.rolesTotal) + "\n")
	}

	structLine := "Structure  "
	allStructDone := len(m.structDone) == len(m.targets) && len(m.targets) > 0
	if allStructDone {
		structLine += doneStyle + " done"
	} else {
		structLine += inProgressStyle
	}
	sb.WriteString(structLine + "\n\n")

	for _, cat := range m.categories {
		totalFetch, totalPost := 0, 0
		for _, id := range cat.channelIDs {
			if s, ok := m.channels[id]; ok {
				totalFetch += s.fetchCount
				for _, v := range s.postCount {
					totalPost += v
				}
			}
		}
		sb.WriteString(fmt.Sprintf("▼ %s   %s / %s msgs posted\n",
			categoryStyle.Render(cat.name),
			formatCount(totalPost),
			formatCount(totalFetch),
		))
		for _, id := range cat.channelIDs {
			s, ok := m.channels[id]
			if !ok {
				continue
			}
			sb.WriteString(m.renderChannelRow(s) + "\n")
		}
		sb.WriteString("\n")
	}

	if m.pipelineErr != nil {
		sb.WriteString("\n" + errorStyle.Render("Pipeline error: "+m.pipelineErr.Error()) + "\n")
	}

	pauseLabel := "[P]ause"
	if m.paused {
		pauseLabel = "[P]Resume"
	}
	sb.WriteString(buttonStyle.Render(pauseLabel) + "  " + buttonStyle.Render("[C]ancel"))
	return sb.String()
}

func (m ProgressModel) renderChannelRow(s *channelState) string {
	status := inProgressStyle
	if s.done {
		status = doneStyle
	} else if s.err != nil {
		status = errorStyle.Render("✗ " + s.err.Error())
	}

	fetchPart := fmt.Sprintf("fetch %s/%s", formatCount(s.fetchCount), formatTotal(s.fetchTotal))
	postParts := ""
	for _, name := range m.targets {
		postParts += fmt.Sprintf("  %s: post %s/%s", name, formatCount(s.postCount[name]), formatTotal(s.postTotal[name]))
	}

	return fmt.Sprintf("    #%-20s %s%s  %s", s.name, fetchPart, postParts, status)
}

func renderBar(label string, done, total int) string {
	const width = 10
	filled := 0
	if total > 0 {
		filled = done * width / total
	}
	bar := strings.Repeat(progressBarFull, filled) + strings.Repeat(progressBarEmpty, width-filled)
	return fmt.Sprintf("%-12s %s  %d / %d", label, bar, done, total)
}

func formatCount(n int) string {
	return fmt.Sprintf("%d", n)
}

func formatTotal(n int) string {
	if n == 0 {
		return "∞"
	}
	return fmt.Sprintf("%d", n)
}
