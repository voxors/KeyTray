package main

import (
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
	slog.Info("hello world!")

	devices := commoninterface.GetAvailableDevices()

	for {
		for _, device := range devices {
			slog.Info(
				"device found",
				"pourcentage", device.BatteryInfo.Pourcentage())
		}
		time.Sleep(1 * time.Second)
	}
}
