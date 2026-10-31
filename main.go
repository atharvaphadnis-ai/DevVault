package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"devvault/internal/events"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	app, err := NewApp(log)
	if err != nil {
		log.Error("failed to initialise DevVault", "err", err)
		fmt.Fprintln(os.Stderr, "failed to initialise DevVault:", err)
		os.Exit(1)
	}
	err = wails.Run(&options.App{
		Title:     "DevVault",
		Width:     1440,
		Height:    900,
		MinWidth:  1280,
		MinHeight: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 11, G: 14, B: 20, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
		Linux: &linux.Options{
			WindowIsTranslucent: false,
		},
		Mac: &mac.Options{},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		log.Error("DevVault exited with error", "err", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// wailsSink forwards service events to the webview.
type wailsSink struct {
	ctx context.Context
}

func (w *wailsSink) Emit(name string, data any) {
	if w.ctx != nil {
		runtime.EventsEmit(w.ctx, name, data)
	}
}

var _ events.Sink = (*wailsSink)(nil)
