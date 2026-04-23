package commoninterface

import (
	"log/slog"

	"github.com/sstallion/go-hid"
	"voxors.org/KeyTray/src/pkg/m3Mouse"
)

type Device struct {
	BatteryInfo BatteryInfo
}

func GetAvailableDevices() []Device {
	var devices []Device
	test := false
	// For now we only need one device info per device for the battery.
	// May not be sufficient in the future
	hid.Enumerate(hid.VendorIDAny, hid.ProductIDAny, func(info *hid.DeviceInfo) error {
		if m3Mouse.CheckHidInfoValid(info) {
			maybeM3MouseBattery := m3Mouse.NewM3MouseBattery(info)
			if maybeM3MouseBattery.IsOk() && !test {
				slog.Info("M3 mouse battery device discovered", "device", info)
				devices = append(devices, Device{BatteryInfo: maybeM3MouseBattery.MustGet()})
				test = true
			}
		}

		return nil
	})

	return devices
}
