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

	devices := device.GetAvailableDevices()
	validDevices := make([]device.Device, 0)
	for _, device := range devices {
		err := device.Driver.Init(context.Background())
		if err != nil {
			slog.Error("Failed to init driver", "device", device, "err", err)
		} else {
			device.Driver.StartBackgroundCheck(context.Background())
			validDevices = append(validDevices, device)
		}
	}

	err = keytray.SetDevices(devices)
	if err != nil {
		slog.Error("Failed to set devices in tray icon", "error", err)
	}
	keytray.StartDeviceWatcher(context.Background())
	tempPercentageSpam := 0
	for {
		for _, device := range validDevices {
			percentage, presence := device.Driver.BatteryPercentage().Get()
			if !presence {
				slog.Debug("Percentage do not exist")
				continue
			}

			if tempPercentageSpam != percentage {
				tempPercentageSpam = percentage

				slog.Info(
					"Battery update",
					"device", device.DeviceName,
					"percentage", percentage)
			}
		}
		time.Sleep(1 * time.Second)
	}
}
