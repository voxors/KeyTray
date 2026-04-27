package main

import (
	"context"
	_ "embed"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/voxors/KeyTray/src/pkg/driver/device"
	"github.com/voxors/KeyTray/src/pkg/tray"
)

//go:embed "assets/keytray.svg"
var logoSvg string

func setupLogger() {
	w := os.Stderr
	slog.SetDefault(slog.New(
		tint.NewHandler(w, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.DateTime,
		}),
	))
}

func main() {
	setupLogger()

	keytray, err := tray.Init().Get()
	if err != nil {
		slog.Error("Failed to create tray icon", "error", err)
	}
	defer func(keytray tray.Keytray) {
		err := keytray.Close()
		if err != nil {
			slog.Error("Failed to close tray icon", "error", err)
		}
	}(keytray)

	err = keytray.SetLogo(logoSvg)
	if err != nil {
		slog.Error("Failed to set icon for tray", "error", err)
	}

	quitChan, err := keytray.AddQuit().Get()
	if err != nil {
		slog.Error("Failed to add quit menu to tray", "error", err)
	}

	deviceWatcher := device.NewDeviceWatcher()
	devicesChan := deviceWatcher.StartDeviceMonitor(context.Background())

waitingLoop:
	for {
		select {
		case devices := <-devicesChan:
			err := keytray.SetDevices(devices)
			if err != nil {
				slog.Error("Failed to set devices for tray", "error", err)
			}
		case <-quitChan:
			break waitingLoop
		}
	}

	slog.Info("KeyTray is shutting down")
}
