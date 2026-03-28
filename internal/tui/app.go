package tui

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/blubskye/discord2stoat/internal/pipeline"
	"github.com/blubskye/discord2stoat/internal/target"
)

type screen int

const (
	screenConfirm   screen = iota
	screenConfigure
	screenProgress
)

// PipelineRunner is called when the user presses Start, with the exported channel configs.
type PipelineRunner func(channelCfgs map[string]pipeline.ChannelConfig)

// AppModel is the top-level bubbletea model.
type AppModel struct {
	current   screen
	confirm   ConfirmModel
	configure ConfigureModel
	progress  ProgressModel

	cancelFn       context.CancelFunc
	pauser         *pipeline.Pauser
	progressCh     chan pipeline.ProgressEvent
	pipelineRunner PipelineRunner
	targetNames    []string
}

// AppConfig holds everything the TUI needs to start.
type AppConfig struct {
	ConfirmEntries []ConfirmEntry
	Channels       []*discordgo.Channel
	Targets        map[string]target.Target
	Discord        pipeline.DiscordFetcher
}

// NewApp creates the root model starting at the confirm screen.
func NewApp(cfg AppConfig) AppModel {
	names := make([]string, 0, len(cfg.Targets))
	for name := range cfg.Targets {
		names = append(names, name)
	}
	return AppModel{
		current:     screenConfirm,
		confirm:     NewConfirmModel(cfg.ConfirmEntries),
		configure:   NewConfigureModel(cfg.Channels),
		targetNames: names,
	}
}

func (m AppModel) Init() tea.Cmd {
	return m.confirm.Init()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case msgConfirmed:
		m.current = screenConfigure
		return m, m.configure.Init()

	case msgStartClone:
		m.current = screenProgress
		return m, m.startPipeline()

	case msgBack:
		m.current = screenConfirm
		return m, m.confirm.Init()
	}

	switch m.current {
	case screenConfirm:
		updated, cmd := m.confirm.Update(msg)
		m.confirm = updated.(ConfirmModel)
		return m, cmd
	case screenConfigure:
		updated, cmd := m.configure.Update(msg)
		m.configure = updated.(ConfigureModel)
		return m, cmd
	case screenProgress:
		updated, cmd := m.progress.Update(msg)
		m.progress = updated.(ProgressModel)
		// Mirror pause state to the pipeline pauser.
		if m.pauser != nil {
			if m.progress.paused {
				m.pauser.Pause()
			} else {
				m.pauser.Resume()
			}
		}
		return m, cmd
	}
	return m, nil
}

func (m AppModel) View() string {
	switch m.current {
	case screenConfirm:
		return m.confirm.View()
	case screenConfigure:
		return m.configure.View()
	case screenProgress:
		return m.progress.View()
	}
	return ""
}

// startPipeline launches the pipeline runner and transitions to the progress screen.
func (m *AppModel) startPipeline() tea.Cmd {
	channelCfgs := m.configure.ExportChannelConfigs()
	if m.pipelineRunner != nil {
		m.pipelineRunner(channelCfgs)
	} else {
		// No runner set: stub for testing.
		go func() {
			defer close(m.progressCh)
			log.Println("pipeline started (stub)")
		}()
	}
	m.progress = buildProgressModel(m.progressCh, m.configure.groups, m.targetNames)
	return m.progress.WaitForEvent()
}

func buildProgressModel(ch chan pipeline.ProgressEvent, groups []CategoryGroup, targetNames []string) ProgressModel {
	channels := make(map[string]*channelState)
	var channelOrder []string
	var categories []categoryProgress

	for _, g := range groups {
		cat := categoryProgress{name: g.Name}
		for _, row := range g.Channels {
			channels[row.DiscordID] = &channelState{
				name:      row.Name,
				postCount: make(map[string]int),
				postTotal: make(map[string]int),
			}
			channelOrder = append(channelOrder, row.DiscordID)
			cat.channelIDs = append(cat.channelIDs, row.DiscordID)
		}
		categories = append(categories, cat)
	}

	return ProgressModel{
		channels:     channels,
		channelOrder: channelOrder,
		categories:   categories,
		targets:      targetNames,
		structDone:   make(map[string]bool),
		progressCh:   ch,
	}
}

// GracefulStop cancels the pipeline context.
func (m *AppModel) GracefulStop() {
	if m.cancelFn != nil {
		m.cancelFn()
	}
	fmt.Println()
}

func (m *AppModel) SetPipelineRunner(fn PipelineRunner) { m.pipelineRunner = fn }
func (m *AppModel) SetProgressCh(ch chan pipeline.ProgressEvent) { m.progressCh = ch }
func (m *AppModel) SetPauser(p *pipeline.Pauser)                 { m.pauser = p }
func (m *AppModel) SetCancel(fn context.CancelFunc)              { m.cancelFn = fn }
