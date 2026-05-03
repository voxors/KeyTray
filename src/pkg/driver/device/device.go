package device

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/samber/lo"
	"github.com/sstallion/go-hid"
	"github.com/voxors/KeyTray/src/pkg/driver/dtype"
	"github.com/voxors/KeyTray/src/pkg/driver/keychronM3"
	"github.com/voxors/KeyTray/src/pkg/driver/keychronM6"
)

type DeviceWatcher struct {
	drivers     []Driver
	unknownList map[hid.DeviceInfo]time.Time
}

func NewDeviceWatcher() DeviceWatcher {
	return DeviceWatcher{
		drivers:     make([]Driver, 0),
		unknownList: map[hid.DeviceInfo]time.Time{},
	}
}

func (dw *DeviceWatcher) GetAvailableDevices() []hid.DeviceInfo {
	var devices []hid.DeviceInfo
	//Filter for device we support
	err := hid.Enumerate(hid.VendorIDAny, hid.ProductIDAny, func(info *hid.DeviceInfo) error {
		if keychronM3.CheckHidInfoValid(*info) {
			devices = append(devices, *info)
		} else if keychronM6.CheckHidInfoValid(*info) {
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
	ch := make(chan []Driver, 1)

	drivers, changed := dw.updateDriverList(ctx)
	if changed {
		ch <- drivers
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				close(ch)
				return
			case <-ticker.C:
				drivers, changed := dw.updateDriverList(ctx)
				if changed {
					ch <- drivers
				}
			}
		}
	}()

	return ch
}

func (dw *DeviceWatcher) Cleanup() {
	for _, device := range dw.drivers {
		device.StopBackgroundCheck()
	}
	dw.drivers = make([]Driver, 0)
}

func (dw *DeviceWatcher) addDeviceInfoOrCreateKeychronM3Driver(ctx context.Context, hidInfo hid.DeviceInfo) {
	isAdded := false
	for _, driver := range dw.drivers {
		if driver.GetDriverType() == dtype.KeychronM3 {
			if err := driver.AddDeviceInfo(ctx, hidInfo); err != nil {
				slog.Error("Failed to add device info", "driver", driver, "hid info", hidInfo)
			} else {
				isAdded = true
				break
			}
		}
	}
	if !isAdded {
		newDriver := keychronM3.NewKeychronM3Driver()
		err := newDriver.AddDeviceInfo(ctx, hidInfo)
		if err != nil {
			slog.Error("Failed to add hid info", "driver", &newDriver, "hid info", hidInfo)
		}
		err = newDriver.Init(ctx)
		if err != nil {
			slog.Error("Failed to init driver", "error", err)
		}
		newDriver.StartBackgroundCheck(ctx)
		dw.drivers = append(dw.drivers, newDriver)
	}
}

func (dw *DeviceWatcher) addDeviceInfoOrCreateKeychronM6Driver(ctx context.Context, hidInfo hid.DeviceInfo) {
	isAdded := false
	for _, driver := range dw.drivers {
		if driver.GetDriverType() == dtype.KeychronM6 {
			if err := driver.AddDeviceInfo(ctx, hidInfo); err != nil {
				slog.Error("Failed to add device info", "driver", driver, "hid info", hidInfo)
			} else {
				isAdded = true
				break
			}
		}
	}
	if !isAdded {
		newDriver := keychronM6.NewKeychronM6Driver()
		err := newDriver.AddDeviceInfo(ctx, hidInfo)
		if err != nil {
			slog.Error("Failed to add hid info", "driver", &newDriver, "hid info", hidInfo)
		}
		err = newDriver.Init(ctx)
		if err != nil {
			slog.Error("Failed to init driver", "error", err)
		}
		newDriver.StartBackgroundCheck(ctx)
		dw.drivers = append(dw.drivers, newDriver)
	}
}

func (dw *DeviceWatcher) removeDeviceInfoOrDeleteDriver(hidInfo hid.DeviceInfo) {
	for i, device := range dw.drivers {
		if slices.ContainsFunc(device.GetDeviceInfo(), func(deviceHidInfo hid.DeviceInfo) bool {
			return hidInfo == deviceHidInfo
		}) {
			device.RemoveDeviceInfo(hidInfo)
			if len(device.GetDeviceInfo()) == 0 {
				dw.drivers = append(dw.drivers[:i], dw.drivers[i+1:]...)
			}
		}
	}
	delete(dw.unknownList, hidInfo)
}

func (dw *DeviceWatcher) addedAndRemovedDeviceInfo(hidInfo []hid.DeviceInfo) ([]hid.DeviceInfo, []hid.DeviceInfo) {
	for unknownDevice, lastAttempt := range dw.unknownList {
		if time.Now().Unix()-lastAttempt.Unix() > 60*60 {
			delete(dw.unknownList, unknownDevice)
		}
	}

	currentList := make([]hid.DeviceInfo, 0)
	currentList = append(currentList, slices.Collect(maps.Keys(dw.unknownList))...)
	for _, device := range dw.drivers {
		currentList = append(currentList, device.GetDeviceInfo()...)
	}

	added := make([]hid.DeviceInfo, 0)
	for _, newInfo := range hidInfo {
		if !slices.ContainsFunc(currentList, func(hidInfo hid.DeviceInfo) bool {
			return newInfo == hidInfo
		}) {
			added = append(added, newInfo)
		}
	}
	removed := make([]hid.DeviceInfo, 0)
	for _, currentInfo := range currentList {
		if !slices.ContainsFunc(hidInfo, func(hidInfo hid.DeviceInfo) bool {
			return currentInfo == hidInfo
		}) {
			removed = append(removed, currentInfo)
		}
	}

	return added, removed
}

func (dw *DeviceWatcher) updateDriverList(ctx context.Context) ([]Driver, bool) {
	changed := false
	addedHidInfoList, removedHidInfoList := dw.addedAndRemovedDeviceInfo(dw.GetAvailableDevices())
	// We want to use add real device first so the init have more chance to be successfull
	slices.SortFunc(addedHidInfoList, func(a hid.DeviceInfo, b hid.DeviceInfo) int {
		if isDongle(a.ProductID) == isDongle(b.ProductID) {
			return 0
		}
		if isDongle(a.ProductID) {
			return 1
		}
		if isDongle(b.ProductID) {
			return -1
		}
		return 0
	})
	if len(addedHidInfoList) != 0 {
		slog.Info("Added HID Info List", "addedHidInfoList", lo.Map(addedHidInfoList, func(info hid.DeviceInfo, _ int) string {
			return fmt.Sprintf("0x%x", info.ProductID)
		}))
	}
	if len(removedHidInfoList) != 0 {
		slog.Info("Removed HID Info List", "removedHidInfoList", lo.Map(removedHidInfoList, func(info hid.DeviceInfo, _ int) string {
			return fmt.Sprintf("0x%x", info.ProductID)
		}))
	}
	for _, hidInfo := range addedHidInfoList {
		if isDongle(hidInfo.ProductID) {
			switch getDongleDevice(hidInfo) {
			case KeychronM3Dongle:
				dw.addDeviceInfoOrCreateKeychronM3Driver(ctx, hidInfo)
				changed = true
			case KeychronM6Dongle:
				dw.addDeviceInfoOrCreateKeychronM6Driver(ctx, hidInfo)
				changed = true
			case Unknown:
				slog.Warn("Dongle not recognized", "Product ID", fmt.Sprintf("0x%x", hidInfo.ProductID))
				dw.unknownList[hidInfo] = time.Now()
				continue
			}
		} else {
			if keychronM3.CheckHidInfoValid(hidInfo) {
				dw.addDeviceInfoOrCreateKeychronM3Driver(ctx, hidInfo)
				changed = true
			} else if keychronM6.CheckHidInfoValid(hidInfo) {
				dw.addDeviceInfoOrCreateKeychronM6Driver(ctx, hidInfo)
				changed = true
			}
		}
	}

	for _, hidInfo := range removedHidInfoList {
		dw.removeDeviceInfoOrDeleteDriver(hidInfo)
		changed = true
	}

	return dw.drivers, changed
}
