package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wmac "github.com/wailsapp/wails/v2/pkg/options/mac"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"procom/internal/bootstrap"
	"procom/internal/frontendbridge"
)

func main() {
	logger := log.New(os.Stdout, "intercom ", log.LstdFlags|log.Lmicroseconds)
	runtimeBundle, err := bootstrap.NewRuntime(logger)
	if err != nil {
		log.Fatalf("initialize runtime: %v", err)
	}

	shell := NewDesktopShell(runtimeBundle, logger)

	if err := wails.Run(&options.App{
		Title:            "INTERCOM",
		Width:            1440,
		Height:           960,
		MinWidth:         1100,
		MinHeight:        720,
		BackgroundColour: &options.RGBA{R: 2, G: 6, B: 23, A: 255},
		AssetServer: &assetserver.Options{
			Assets: os.DirFS("frontend/dist"),
		},
		Mac: &wmac.Options{
			TitleBar: wmac.TitleBarHiddenInset(),
		},
		OnStartup:  shell.Startup,
		OnShutdown: shell.Shutdown,
		Bind: []interface{}{
			&FrontendBridge{bridge: runtimeBundle.Bridge},
		},
	}); err != nil {
		log.Fatalf("run desktop shell: %v", err)
	}
}

type DesktopShell struct {
	runtime *bootstrap.Runtime
	logger  *log.Logger
	cancel  context.CancelFunc
	done    chan error
}

func NewDesktopShell(runtimeBundle *bootstrap.Runtime, logger *log.Logger) *DesktopShell {
	return &DesktopShell{runtime: runtimeBundle, logger: logger}
}

func (s *DesktopShell) Startup(ctx context.Context) {
	s.runtime.Bridge.AttachEmitter(wailsEmitter{ctx: ctx})
	runCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan error, 1)
	go func() {
		s.done <- s.runtime.App.Run(runCtx)
	}()
}

func (s *DesktopShell) Shutdown(context.Context) {
	if s.cancel != nil {
		s.cancel()
	}
	if s.done != nil {
		if err := <-s.done; err != nil && !errors.Is(err, context.Canceled) {
			s.logger.Printf("runtime shutdown error: %v", err)
		}
	}
}

type wailsEmitter struct {
	ctx context.Context
}

func (e wailsEmitter) Emit(eventName string, payload any) {
	wruntime.EventsEmit(e.ctx, eventName, payload)
}

type FrontendBridge struct {
	bridge *frontendbridge.Bridge
}

func (b *FrontendBridge) GetBootstrap() (frontendbridge.BootstrapPayload, error) {
	return b.bridge.GetBootstrap()
}

func (b *FrontendBridge) UpdateChannel(input frontendbridge.ChannelUpdateInput) (frontendbridge.Channel, error) {
	return b.bridge.UpdateChannel(input)
}

func (b *FrontendBridge) AddChannel(input frontendbridge.ChannelAddInput) (frontendbridge.Channel, error) {
	return b.bridge.AddChannel(input)
}

func (b *FrontendBridge) RemoveChannel(channelID string) error {
	return b.bridge.RemoveChannel(channelID)
}

func (b *FrontendBridge) UpdateKeywords(rules []frontendbridge.KeywordRuleInput) error {
	return b.bridge.UpdateKeywords(rules)
}

func (b *FrontendBridge) UpdateOSC(input frontendbridge.OSCSettingsInput) error {
	return b.bridge.UpdateOSC(input)
}

var _ fs.FS = os.DirFS(".")
