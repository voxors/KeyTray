package device

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/sstallion/go-hid"
	"github.com/voxors/KeyTray/src/pkg/driver/keychronM3"
)

type DeviceWatcher struct {
	drivers []Driver
}

func NewDeviceWatcher() DeviceWatcher {
	return DeviceWatcher{
		drivers: []Driver{},
	}
}

func (dw *DeviceWatcher) GetAvailableDevices() []hid.DeviceInfo {
	var devices []hid.DeviceInfo
	//Filter for device we support
	err := hid.Enumerate(hid.VendorIDAny, hid.ProductIDAny, func(info *hid.DeviceInfo) error {
		if keychronM3.CheckHidInfoValid(info) {
			devices = append(devices, *info)
		}
		return nil
	})

	if err != nil {
		slog.Error("Failed to enumerate HID devices", "error", err)
	}

	return devices
}

func (dw *DeviceWatcher) StartDeviceMonitor(ctx context.Context) <-chan []Driver {
	ch := make(chan []Driver)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				close(ch)
				return
			case <-ticker.C:
				ch <- dw.updateDriverList(ctx)
			}
		}
	}()

	return ch
}

func (dw *DeviceWatcher) updateDriverList(ctx context.Context) []Driver {
	drivers := make([]Driver, 0)
	hidDevicesList := dw.GetAvailableDevices()
	for _, hidDevice := range hidDevicesList {
		isDriverRunning := false
		for _, driver := range dw.drivers {
			if slices.Contains(driver.GetProductID(), int(hidDevice.ProductID)) ||
				slices.Contains(driver.GetVendorID(), int(hidDevice.VendorID)) {
				isDriverRunning = true
			}
		}
		for _, driver := range drivers {
			if slices.Contains(driver.GetProductID(), int(hidDevice.ProductID)) ||
				slices.Contains(driver.GetVendorID(), int(hidDevice.VendorID)) {
				isDriverRunning = true
			}
		}
		if !isDriverRunning {
			if keychronM3.CheckHidInfoValid(&hidDevice) {
				maybeKeychronM3Driver := keychronM3.NewKeychronM3Driver(&hidDevice)
				if maybeKeychronM3Driver.IsOk() {
					slog.Info(
						"Keychron M3 mouse discovered",
						"Vendor ID", fmt.Sprintf("0x%x", hidDevice.VendorID),
						"Product ID", fmt.Sprintf("0x%x", hidDevice.ProductID),
					)

					err := maybeKeychronM3Driver.MustGet().Init(ctx)
					if err != nil {
						slog.Error("Failed to initialize Keychron M3 mouse driver", "error", err.Error())
					}
					maybeKeychronM3Driver.MustGet().StartBackgroundCheck(ctx)
					drivers = append(drivers, maybeKeychronM3Driver.MustGet())
				}
			}
		}
	}

driverLoop:
	for _, driver := range dw.drivers {
		for _, hidDevice := range hidDevicesList {
			if slices.Contains(driver.GetProductID(), int(hidDevice.ProductID)) ||
				slices.Contains(driver.GetVendorID(), int(hidDevice.VendorID)) {
				drivers = append(drivers, driver)
				continue driverLoop
			}
		}
	}

	dw.drivers = drivers
	return drivers
}
