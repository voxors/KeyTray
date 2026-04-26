package tray

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"strings"

	"deedles.dev/tray"
	"github.com/samber/mo"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"github.com/voxors/KeyTray/src/pkg/driver/device"
)

const (
	logoSizePixmap = 128
)

type Keytray struct {
	item    *tray.Item
	devices []device.Device
	logo    mo.Option[*image.RGBA]
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
		item:    item,
		devices: []device.Device{},
		logo:    mo.None[*image.RGBA](),
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
				case <-updates:
					err := k.updateTray()
					if err != nil {
						slog.Warn("Failed to update tray")
					}
				}
			}
		}(dev, updates)
	}
}

func (k *Keytray) updateTray() error {
	var tooltipTexts []string
	lowestPercentage := mo.None[int]()
	for _, dev := range k.devices {
		percentage, exist := dev.Driver.BatteryPercentage().Get()
		if exist {
			tooltipTexts = append(tooltipTexts, fmt.Sprintf("%s: %d%%", dev.DeviceName, percentage))
		} else {
			tooltipTexts = append(tooltipTexts, fmt.Sprintf("%s: %s", dev.DeviceName, "Unavailable"))
		}

		if lowestPercentage.IsNone() {
			lowestPercentage = mo.Some(percentage)
		} else if percentage < lowestPercentage.MustGet() {
			lowestPercentage = mo.Some(percentage)
		}
	}

	var iconName string
	if lowestPercentage.IsSome() {
		iconNamePercentage := (lowestPercentage.MustGet() % 10) * 10
		iconName = fmt.Sprintf("battery-%03d", iconNamePercentage)
	} else {
		if k.logo.IsSome() {
			err := k.item.SetProps(
				tray.ItemIconPixmap(k.logo.MustGet()),
			)
			if err != nil {
				slog.Warn("Failed to set logo", "error", err)
			}
		}
	}
	err := k.item.SetProps(
		tray.ItemToolTip("", nil, "Keytray", strings.Join(tooltipTexts, "\n")),
		tray.ItemIconName(iconName),
	)
	if err != nil {
		return err
	}

	return nil
}

func (k *Keytray) SetDevices(devices []device.Device) error {
	k.devices = devices
	err := k.updateTray()
	if err != nil {
		return err
	}
	return nil
}

func (k *Keytray) SetLogo(svgContent string) error {
	svgIcon, err := oksvg.ReadIconStream(strings.NewReader(svgContent))
	if err != nil {
		return err
	}

	rectangle := image.Rect(0, 0, logoSizePixmap, logoSizePixmap)
	image := image.NewRGBA(rectangle)
	dasher := rasterx.NewDasher(
		logoSizePixmap,
		logoSizePixmap,
		rasterx.NewScannerGV(
			logoSizePixmap,
			logoSizePixmap,
			image,
			rectangle,
		),
	)
	svgIcon.SetTarget(0, 0, logoSizePixmap, logoSizePixmap)
	svgIcon.Draw(dasher, 1.0)

	k.logo = mo.Some(image)

	return nil
}

func (k *Keytray) Close() error {
	return k.item.Close()
}
