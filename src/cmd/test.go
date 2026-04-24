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
	devices := commoninterface.GetAvailableDevices()
	tempPourcentageSpam := 0
	for {
		for _, device := range devices {
			pourcentage, err := device.BatteryInfo.Pourcentage().Get()
			if err != nil {
				slog.Error("error", "msg", err)
				continue
			}

			if tempPourcentageSpam != pourcentage {
				tempPourcentageSpam = pourcentage

				slog.Info(
					"device found",
					"pourcentage", pourcentage)
			}
		}
		time.Sleep(1 * time.Second)
	}
}
