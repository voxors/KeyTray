package tray

import (
	"context"
	"fmt"
	"log/slog"

	"deedles.dev/tray"
	"github.com/samber/mo"
	"github.com/voxors/KeyTray/src/pkg/driver/device"
)

type Keytray struct {
	item    *tray.Item
	devices []device.Device
}

func Init() mo.Result[Keytray] {
	item, err := tray.New(
		tray.ItemID("Keytray"),
		tray.ItemTitle("Keytray"),
	)
	if err != nil {
		return mo.Err[Keytray](err)
	}

	keytray := Keytray{
		item: item,
	}

	return mo.Ok(keytray)
}

func (k *Keytray) StartDeviceWatcher(ctx context.Context) {
	for _, dev := range k.devices {
		updates := dev.Driver.SubscribeBatteryPercentage()
		go func(d device.Device, updates chan int) {
			for {
				select {
				case <-ctx.Done():
					d.Driver.UnsubscribeBatteryPercentage(updates)
					return
				case percentage := <-updates:
					err := k.updateTray(d, percentage)
					if err != nil {
						slog.Warn("Failed to update tray")
					}
				}
			}
		}(dev, updates)
	}
}

func (k *Keytray) updateTray(dev device.Device, percentage int) error {
	title := fmt.Sprintf("%s: %d%%", dev.DeviceName, percentage)
	iconNamePercentage := (percentage % 10) * 10
	err := k.item.SetProps(
		tray.ItemToolTip("", nil, "Keytray", title),
		tray.ItemIconName(fmt.Sprintf("battery-%03d", iconNamePercentage)),
	)
	if err != nil {
		return err
	}

	return nil
}

func (k *Keytray) SetDevices(devices []device.Device) error {
	k.devices = devices
	for _, dev := range k.devices {
		percentage := dev.Driver.BatteryPercentage()
		if percentage.IsPresent() {
			err := k.updateTray(dev, percentage.MustGet())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (k *Keytray) Close() error {
	return k.item.Close()
}
