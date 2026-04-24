package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	commoninterface "voxors.org/KeyTray/src/pkg/commonInterface"
)

func setupLogger() {
	w := os.Stderr
	slog.SetDefault(slog.New(
		tint.NewHandler(w, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		}),
	))
}

func main() {
	setupLogger()
	devices := commoninterface.GetAvailableDevices()
	validDevices := make([]commoninterface.Device, 0)
	for _, device := range devices {
		err := device.Driver.Init(context.Background())
		if err != nil {
			slog.Error("Failed to init driver", "device", device, "err", err)
		} else {
			device.Driver.StartBackgroundCheck(context.Background())
			validDevices = append(validDevices, device)
		}
	}
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
