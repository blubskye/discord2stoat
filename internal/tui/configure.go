// Copyright (C) 2026 blubskye <https://github.com/blubskye/discord2stoat>
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/blubskye/discord2stoat/internal/pipeline"
)

var (
	categoryStyle  = lipgloss.NewStyle().Bold(true)
	channelStyle   = lipgloss.NewStyle().PaddingLeft(4)
	overrideMarker = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(" *")
	cursorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	selectAllStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	skipStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("160")).Bold(true)
	skipDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

// ChanConfig is the user-editable config for one channel.
type ChanConfig struct {
	Attribution  pipeline.AttributionMode
	MessageLimit int  // 0 = all
	Skip         bool // true = don't create or mirror this channel
	Overridden   bool // true if manually changed from category default
}

// CategoryGroup holds a category and its channels for display.
type CategoryGroup struct {
	DiscordID  string
	Name       string
	Collapsed  bool
	Config     ChanConfig
	Overridden bool // true if this category's config was manually set
	Channels   []ChannelRow
}

// ChannelRow is one text or voice channel in the list.
type ChannelRow struct {
	DiscordID string
	Name      string
	Config    ChanConfig
}

// configHeaderLines and configFooterLines are the fixed line counts above/below
// the scrollable content area in the configure screen.
const configHeaderLines = 2 // title + blank line
const configFooterLines = 3 // blank + buttons line + help text line

// ConfigureModel is the bubbletea model for Screen 2.
type ConfigureModel struct {
	// selectAll holds the global default.
	selectAll ChanConfig
	groups    []CategoryGroup
	// flat is the flattened list of focusable rows: selectAll row, category rows, channel rows.
	flat         []configRow
	cursor       int
	editField    int // 0=attribution, 1=limit
	editingLimit bool
	limitInput   string
	scrollOffset int
	termHeight   int
}

type configRowKind int

const (
	rowSelectAll configRowKind = iota
	rowCategory
	rowChannel
)

type configRow struct {
	kind       configRowKind
	groupIdx   int // index into groups
	channelIdx int // index into group.Channels (-1 for category/selectAll rows)
}

// NewConfigureModel builds Screen 2 from Discord channel data.
func NewConfigureModel(channels []*discordgo.Channel) ConfigureModel {
	catMap := map[string]*CategoryGroup{}
	var catOrder []string
	var uncategorized CategoryGroup
	uncategorized.Name = "Uncategorized"

	for _, ch := range channels {
		if ch.Type == discordgo.ChannelTypeGuildCategory {
			g := &CategoryGroup{
				DiscordID: ch.ID,
				Name:      ch.Name,
				Config:    ChanConfig{Attribution: pipeline.AttributionPrefix},
			}
			catMap[ch.ID] = g
			catOrder = append(catOrder, ch.ID)
		}
	}
	for _, ch := range channels {
		if ch.Type != discordgo.ChannelTypeGuildText && ch.Type != discordgo.ChannelTypeGuildVoice {
			continue
		}
		row := ChannelRow{
			DiscordID: ch.ID,
			Name:      ch.Name,
			Config:    ChanConfig{Attribution: pipeline.AttributionPrefix},
		}
		if g, ok := catMap[ch.ParentID]; ok {
			g.Channels = append(g.Channels, row)
		} else {
			uncategorized.Channels = append(uncategorized.Channels, row)
		}
	}

	groups := make([]CategoryGroup, 0, len(catOrder)+1)
	for _, id := range catOrder {
		groups = append(groups, *catMap[id])
	}
	if len(uncategorized.Channels) > 0 {
		groups = append(groups, uncategorized)
	}

	m := ConfigureModel{
		selectAll: ChanConfig{Attribution: pipeline.AttributionPrefix},
		groups:    groups,
	}
	m.rebuildFlat()
	return m
}

func (m *ConfigureModel) rebuildFlat() {
	m.flat = []configRow{{kind: rowSelectAll, groupIdx: -1, channelIdx: -1}}
	for gi, g := range m.groups {
		m.flat = append(m.flat, configRow{kind: rowCategory, groupIdx: gi, channelIdx: -1})
		if !g.Collapsed {
			for ci := range g.Channels {
				m.flat = append(m.flat, configRow{kind: rowChannel, groupIdx: gi, channelIdx: ci})
			}
		}
	}
}

// ensureVisible adjusts scrollOffset so the cursor row is within the visible window.
func (m *ConfigureModel) ensureVisible() {
	if m.termHeight == 0 {
		return
	}
	available := m.termHeight - configHeaderLines - configFooterLines
	if available < 1 {
		available = 1
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+available {
		m.scrollOffset = m.cursor - available + 1
	}
}

func (m ConfigureModel) Init() tea.Cmd { return nil }

func (m ConfigureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	row := m.flat[m.cursor]

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termHeight = msg.Height
		m.ensureVisible()

	case tea.KeyMsg:
		if m.editingLimit {
			switch msg.String() {
			case "enter":
				m.applyLimitInput(row)
				m.editingLimit = false
				m.limitInput = ""
			case "backspace":
				if len(m.limitInput) > 0 {
					m.limitInput = m.limitInput[:len(m.limitInput)-1]
				}
			case "esc":
				m.editingLimit = false
				m.limitInput = ""
			default:
				if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
					m.limitInput += msg.String()
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		case "down", "j":
			if m.cursor < len(m.flat)-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "pgup":
			step := 10
			if m.cursor >= step {
				m.cursor -= step
			} else {
				m.cursor = 0
			}
			m.ensureVisible()
		case "pgdown":
			step := 10
			if m.cursor+step < len(m.flat) {
				m.cursor += step
			} else {
				m.cursor = len(m.flat) - 1
			}
			m.ensureVisible()
		case "left", "h":
			if row.kind == rowCategory {
				m.groups[row.groupIdx].Collapsed = true
				m.rebuildFlat()
			} else if m.editField > 0 {
				m.editField--
			}
		case "right", "l":
			if row.kind == rowCategory {
				m.groups[row.groupIdx].Collapsed = false
				m.rebuildFlat()
			} else if m.editField < 1 {
				m.editField++
			}
		case "tab":
			m.editField = (m.editField + 1) % 2
		case "enter", " ":
			m.toggleField(row)
		case "x":
			m.toggleSkip(row)
		case "s":
			return m, func() tea.Msg { return msgStartClone{} }
		case "b":
			return m, func() tea.Msg { return msgBack{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

// toggleSkip flips the Skip flag on the current row and propagates to children.
func (m *ConfigureModel) toggleSkip(row configRow) {
	cfg := m.getConfig(row)
	cfg.Skip = !cfg.Skip
	m.setConfig(row, cfg, true)
	if row.kind == rowCategory || row.kind == rowSelectAll {
		m.propagateSkipToChildren(row, cfg.Skip)
	}
}

// propagateSkipToChildren sets Skip on all non-overridden children of a category/selectAll row.
func (m *ConfigureModel) propagateSkipToChildren(row configRow, skip bool) {
	if row.kind == rowSelectAll {
		for gi := range m.groups {
			if !m.groups[gi].Overridden {
				m.groups[gi].Config.Skip = skip
				for ci := range m.groups[gi].Channels {
					if !m.groups[gi].Channels[ci].Config.Overridden {
						m.groups[gi].Channels[ci].Config.Skip = skip
					}
				}
			}
		}
		return
	}
	for ci := range m.groups[row.groupIdx].Channels {
		if !m.groups[row.groupIdx].Channels[ci].Config.Overridden {
			m.groups[row.groupIdx].Channels[ci].Config.Skip = skip
		}
	}
}

func (m *ConfigureModel) toggleField(row configRow) {
	cfg := m.getConfig(row)
	if m.editField == 0 {
		if cfg.Attribution == pipeline.AttributionPrefix {
			cfg.Attribution = pipeline.AttributionContentOnly
		} else {
			cfg.Attribution = pipeline.AttributionPrefix
		}
		m.setConfig(row, cfg, true)
		if row.kind == rowCategory || row.kind == rowSelectAll {
			m.propagateToChildren(row)
		}
	} else {
		m.editingLimit = true
		if cfg.MessageLimit > 0 {
			m.limitInput = fmt.Sprintf("%d", cfg.MessageLimit)
		}
	}
}

func (m *ConfigureModel) applyLimitInput(row configRow) {
	cfg := m.getConfig(row)
	if m.limitInput == "" || m.limitInput == "0" {
		cfg.MessageLimit = 0
	} else {
		var n int
		fmt.Sscanf(m.limitInput, "%d", &n)
		cfg.MessageLimit = n
	}
	m.setConfig(row, cfg, true)
	if row.kind == rowCategory || row.kind == rowSelectAll {
		m.propagateToChildren(row)
	}
}

func (m *ConfigureModel) propagateToChildren(row configRow) {
	parentCfg := m.getConfig(row)
	if row.kind == rowSelectAll {
		for gi := range m.groups {
			if !m.groups[gi].Overridden {
				m.groups[gi].Config = parentCfg
				for ci := range m.groups[gi].Channels {
					if !m.groups[gi].Channels[ci].Config.Overridden {
						m.groups[gi].Channels[ci].Config = parentCfg
					}
				}
			}
		}
		return
	}
	for ci := range m.groups[row.groupIdx].Channels {
		if !m.groups[row.groupIdx].Channels[ci].Config.Overridden {
			m.groups[row.groupIdx].Channels[ci].Config = parentCfg
		}
	}
}

func (m *ConfigureModel) getConfig(row configRow) ChanConfig {
	switch row.kind {
	case rowSelectAll:
		return m.selectAll
	case rowCategory:
		return m.groups[row.groupIdx].Config
	default:
		return m.groups[row.groupIdx].Channels[row.channelIdx].Config
	}
}

func (m *ConfigureModel) setConfig(row configRow, cfg ChanConfig, markOverride bool) {
	switch row.kind {
	case rowSelectAll:
		m.selectAll = cfg
	case rowCategory:
		if markOverride {
			m.groups[row.groupIdx].Overridden = true
		}
		m.groups[row.groupIdx].Config = cfg
	default:
		if markOverride {
			cfg.Overridden = true
		}
		m.groups[row.groupIdx].Channels[row.channelIdx].Config = cfg
	}
}

// ExportChannelConfigs returns pipeline.ChannelConfig for each Discord channel ID.
func (m ConfigureModel) ExportChannelConfigs() map[string]pipeline.ChannelConfig {
	out := make(map[string]pipeline.ChannelConfig)
	for _, g := range m.groups {
		for _, ch := range g.Channels {
			out[ch.DiscordID] = pipeline.ChannelConfig{
				Attribution:  ch.Config.Attribution,
				MessageLimit: ch.Config.MessageLimit,
				Skip:         ch.Config.Skip,
			}
		}
	}
	return out
}

func (m ConfigureModel) View() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Configure clone") + "\n\n")

	// Determine the visible window of rows.
	available := len(m.flat)
	if m.termHeight > 0 {
		avail := m.termHeight - configHeaderLines - configFooterLines
		if avail < 1 {
			avail = 1
		}
		available = avail
	}
	start := m.scrollOffset
	end := start + available
	if end > len(m.flat) {
		end = len(m.flat)
	}

	for i := start; i < end; i++ {
		row := m.flat[i]
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▶ ")
		}

		switch row.kind {
		case rowSelectAll:
			cfg := m.selectAll
			sb.WriteString(cursor + selectAllStyle.Render("[Select All]") + "  " + m.renderFields(cfg, i == m.cursor) + "\n")
		case rowCategory:
			g := m.groups[row.groupIdx]
			arrow := "▼"
			if g.Collapsed {
				arrow = "▶"
			}
			catLabel := fmt.Sprintf("%s %s", arrow, g.Name)
			if g.Config.Skip {
				sb.WriteString(cursor + skipDimStyle.Render(categoryStyle.Render(catLabel)) + "  " + skipStyle.Render("[SKIP]") + "\n")
			} else {
				sb.WriteString(cursor + categoryStyle.Render(catLabel) + "  " + m.renderFields(g.Config, i == m.cursor) + "\n")
			}
		case rowChannel:
			ch := m.groups[row.groupIdx].Channels[row.channelIdx]
			override := ""
			if ch.Config.Overridden {
				override = overrideMarker
			}
			name := "#" + ch.Name
			if ch.Config.Skip {
				sb.WriteString(cursor + skipDimStyle.Render(channelStyle.Render(name)) + "  " + skipStyle.Render("[SKIP]") + override + "\n")
			} else {
				sb.WriteString(cursor + channelStyle.Render(name) + "  " + m.renderFields(ch.Config, i == m.cursor) + override + "\n")
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(buttonStyle.Render("[S]tart") + "  " + buttonStyle.Render("[B]ack") + "  " + buttonStyle.Render("[Q]uit"))
	sb.WriteString("\n  ↑↓/j/k or PgUp/PgDn scroll · ← → or Tab switch fields · Enter toggle/edit · X skip channel · ← → on category collapse")
	return sb.String()
}

func (m ConfigureModel) renderFields(cfg ChanConfig, active bool) string {
	attrLabel := "Prefix"
	if cfg.Attribution == pipeline.AttributionContentOnly {
		attrLabel = "Content"
	}
	limitLabel := "All"
	if cfg.MessageLimit > 0 {
		limitLabel = fmt.Sprintf("Last %d", cfg.MessageLimit)
	}

	attrField := fmt.Sprintf("Attribution: [%s]", attrLabel)
	limitField := fmt.Sprintf("Messages: [%s]", limitLabel)
	if active && m.editingLimit && m.editField == 1 {
		limitField = fmt.Sprintf("Messages: [%s_]", m.limitInput)
	}

	af := lipgloss.NewStyle()
	lf := lipgloss.NewStyle()
	if active {
		if m.editField == 0 {
			af = af.Foreground(lipgloss.Color("63")).Bold(true)
		} else {
			lf = lf.Foreground(lipgloss.Color("63")).Bold(true)
		}
	}
	return af.Render(attrField) + "  " + lf.Render(limitField)
}
