package device

import (
	"fmt"
	"log/slog"

	"github.com/sstallion/go-hid"
	"github.com/voxors/KeyTray/src/pkg/driver/keychronM3"
)

type Device struct {
	DeviceName string
	Driver     Driver
}

func GetAvailableDevices() []Device {
	var devices []Device
	// The Keychron M3 mouse support use multiple device
	// there is a risk that a user could plug two Keychron M3
	// and finding the mismatched dongle and device. But it minimal compared
	// to the pain of dealing with finding the correct match between
	// and maybe having two Keychron M3 instance fighting over the same device.
	// so, we at least ensure that we don't create multiple instance of the Keychron M3
	keychronM3MouseFound := false
	hid.Enumerate(hid.VendorIDAny, hid.ProductIDAny, func(info *hid.DeviceInfo) error {
		if keychronM3.CheckHidInfoValid(info) {
			maybeKeychronM3Driver := keychronM3.NewKeychronM3Driver(info)
			if maybeKeychronM3Driver.IsOk() && !keychronM3MouseFound {
				slog.Info(
					"Keychron M3 mouse discovered",
					"Vendor ID", fmt.Sprintf("0x%x", info.VendorID),
					"Product ID", fmt.Sprintf("0x%x", info.ProductID),
				)
				devices = append(devices, Device{
					DeviceName: "Keychron M3",
					Driver:     maybeKeychronM3Driver.MustGet(),
				})
				keychronM3MouseFound = true
			}
		}

		return nil
	})

	return devices
}
