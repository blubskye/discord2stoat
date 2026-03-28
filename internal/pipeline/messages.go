package pipeline

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/blubskye/discord2stoat/internal/normalized"
	"github.com/blubskye/discord2stoat/internal/target"
)

// ChannelConfig holds the user's per-channel configuration from the TUI.
type ChannelConfig struct {
	Attribution  AttributionMode
	MessageLimit int // 0 = all
}

// AttributionMode controls how message author is formatted.
type AttributionMode int

const (
	AttributionPrefix      AttributionMode = iota // "[Username]: content"
	AttributionContentOnly                        // "content"
)

// Pauser allows workers to check for a pause signal between operations.
type Pauser struct {
	mu     sync.Mutex
	paused bool
	resume chan struct{}
}

func NewPauser() *Pauser {
	return &Pauser{resume: make(chan struct{})}
}

// Pause signals all workers to pause.
func (p *Pauser) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = true
}

// Resume unblocks all paused workers.
func (p *Pauser) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.paused {
		return
	}
	p.paused = false
	close(p.resume)
	p.resume = make(chan struct{})
}

// Check blocks if paused; returns ctx.Err() if context is cancelled while waiting.
func (p *Pauser) Check(ctx context.Context) error {
	p.mu.Lock()
	if !p.paused {
		p.mu.Unlock()
		return nil
	}
	ch := p.resume
	p.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		return nil
	}
}

// RunPhaseB starts one fetch+post goroutine pair per text channel.
// It returns when all channels are done or ctx is cancelled.
func RunPhaseB(
	ctx context.Context,
	discordClient interface {
		FetchMessages(channelID string, limit int, out chan<- *discordgo.Message, done <-chan struct{}) error
	},
	targets map[string]target.Target,        // targetName → Target
	channelMaps map[string]IDMap,             // targetName → discordChannelID→targetChannelID
	textChannels []*discordgo.Channel,
	channelCfg map[string]ChannelConfig,      // discordChannelID → config
	progressCh chan<- ProgressEvent,
	pauser *Pauser,
) {
	var wg sync.WaitGroup

	for _, ch := range textChannels {
		ch := ch // capture
		cfg, ok := channelCfg[ch.ID]
		if !ok {
			cfg = ChannelConfig{Attribution: AttributionPrefix}
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runChannelWorker(ctx, discordClient, targets, channelMaps, ch, cfg, progressCh, pauser); err != nil {
				log.Printf("channel %s (%s): %v", ch.Name, ch.ID, err)
				progressCh <- ProgressEvent{
					Kind:      EventChannelError,
					ChannelID: ch.ID,
					Err:       err,
				}
				// EventChannelError is terminal; send EventChannelDone so the TUI can
				// mark the channel complete regardless of the error path.
				progressCh <- ProgressEvent{Kind: EventChannelDone, ChannelID: ch.ID}
			}
		}()
	}

	wg.Wait()
	for name := range targets {
		progressCh <- ProgressEvent{Kind: EventPhaseBDone, TargetName: name}
	}
}

func runChannelWorker(
	ctx context.Context,
	discordClient interface {
		FetchMessages(channelID string, limit int, out chan<- *discordgo.Message, done <-chan struct{}) error
	},
	targets map[string]target.Target,
	channelMaps map[string]IDMap,
	ch *discordgo.Channel,
	cfg ChannelConfig,
	progressCh chan<- ProgressEvent,
	pauser *Pauser,
) error {
	buf := make(chan *discordgo.Message, 500)
	// workerCtx lets the post loop cancel the fetch goroutine on early exit.
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	// Fetch goroutine: reads from Discord and pushes to buf.
	fetchErr := make(chan error, 1)
	go func() {
		defer close(buf)
		fetchErr <- discordClient.FetchMessages(ch.ID, cfg.MessageLimit, buf, workerCtx.Done())
	}()

	// Post goroutine: reads from buf and sends to all targets.
	fetched := 0
	posted := map[string]int{}

	for msg := range buf {
		if err := pauser.Check(ctx); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fetched++
		progressCh <- ProgressEvent{
			Kind:      EventChannelFetch,
			ChannelID: ch.ID,
			Count:     fetched,
			Total:     cfg.MessageLimit,
		}

		authorName := ""
		if cfg.Attribution == AttributionPrefix && msg.Author != nil {
			authorName = msg.Author.Username
		}

		// Download attachments (images, videos, files) from Discord CDN.
		// Attachment bytes are read-only shared across all targets; adapters must not mutate Data.
		var attachments []normalized.Attachment
		for _, a := range msg.Attachments {
			if a.Size > 100*1024*1024 { // skip files > 100 MB
				log.Printf("skipping large attachment %s (%d bytes)", a.Filename, a.Size)
				continue
			}
			req, err := http.NewRequestWithContext(workerCtx, http.MethodGet, a.URL, nil)
			if err != nil {
				log.Printf("build request for attachment %s: %v", a.Filename, err)
				continue
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("download attachment %s: %v", a.Filename, err)
				continue
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				log.Printf("download attachment %s: HTTP %d", a.Filename, resp.StatusCode)
				continue
			}
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Printf("read attachment %s: %v", a.Filename, err)
				continue
			}
			attachments = append(attachments, normalized.Attachment{
				Filename: a.Filename,
				Data:     data,
			})
		}

		norm := normalized.Message{
			Content:     msg.Content,
			AuthorName:  authorName,
			Timestamp:   msg.Timestamp,
			Attachments: attachments,
		}

		for name, t := range targets {
			targetChanID := channelMaps[name][ch.ID]
			if targetChanID == "" {
				continue
			}
			if err := t.SendMessage(targetChanID, norm); err != nil {
				return fmt.Errorf("[%s] SendMessage: %w", name, err)
			}
			posted[name]++
			progressCh <- ProgressEvent{
				Kind:       EventChannelPost,
				TargetName: name,
				ChannelID:  ch.ID,
				Count:      posted[name],
				Total:      cfg.MessageLimit,
			}
		}
	}

	if err := <-fetchErr; err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	progressCh <- ProgressEvent{Kind: EventChannelDone, ChannelID: ch.ID}
	return nil
}
