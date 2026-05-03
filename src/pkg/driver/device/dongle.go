package device

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/samber/mo"
	"github.com/sstallion/go-hid"
	"github.com/voxors/KeyTray/src/pkg/driver/keychronM3"
	"github.com/voxors/KeyTray/src/pkg/driver/keychronM6"
	"github.com/voxors/KeyTray/src/pkg/usb"
)

const (
	dongleAUsagePage = 0xFF60
)

const (
	Unknown = iota
	KeychronM3Dongle
	KeychronM6Dongle
)

var (
	ErrInvalidResponseFromDongle = errors.New("invalid response from dongle")

	dongleProductIDA = []uint16{
		keychronM3.PRODUCT_ID_DONGLE,
	}
	dongleProductIDB = []uint16{
		keychronM6.PRODUCT_ID_DONGLE_USB_A,
		keychronM6.PRODUCT_ID_DONGLE_USB_C,
	}
)

func isDongle(productID uint16) bool {
	return slices.Contains(append(dongleProductIDA, dongleProductIDB...), productID)
}

func getProductIDOfDongleA(dongleInfo hid.DeviceInfo) mo.Result[int] {
	targetRoot := usb.GetHardwareID(dongleInfo.Path)
	var enumaretedDongleInfo hid.DeviceInfo
	err := hid.Enumerate(dongleInfo.VendorID, dongleInfo.ProductID, func(info *hid.DeviceInfo) error {
		if info.UsagePage == dongleAUsagePage {
			currentRoot := usb.GetHardwareID(info.Path)
			if currentRoot == targetRoot {
				enumaretedDongleInfo = *info
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return mo.Err[int](err)
	}

	dongle, err := hid.OpenPath(enumaretedDongleInfo.Path)
	if err != nil {
		return mo.Err[int](err)
	}
	defer func(device hid.Device) {
		err := device.Close()
		if err != nil {
			slog.Error("Failed to close dongle", "device", device)
		}
	}(*dongle)

	buffer := make([]byte, 33)
	buffer[1] = 0xb2
	_, err = dongle.SendOutputReport(buffer)
	if err != nil {
		return mo.Err[int](err)
	}

	_, err = dongle.ReadWithTimeout(buffer, 1*time.Second)
	if err != hid.ErrTimeout && err != nil {
		return mo.Err[int](err)
	}

	if buffer[0] == 0xb2 {
		return mo.Ok(int(binary.LittleEndian.Uint16(buffer[4:6])))
	}

	buffer = make([]byte, 33)
	buffer[1] = 0x01
	_, err = dongle.SendOutputReport(buffer)
	if err != nil {
		return mo.Err[int](err)
	}

	_, err = dongle.ReadWithTimeout(buffer, 1*time.Second)
	if err != nil {
		return mo.Err[int](err)
	}

	if buffer[0] != 0x1 || buffer[3] != 0x34 || buffer[4] != 0x34 {
		return mo.Err[int](ErrInvalidResponseFromDongle)
	}

	return mo.Ok(int(binary.BigEndian.Uint16(buffer[5:7])))
}

func getProductIDOfDongleB(dongleInfo hid.DeviceInfo) mo.Result[int] {
	targetRoot := usb.GetHardwareID(dongleInfo.Path)
	var enumaretedDongleInfo hid.DeviceInfo
	err := hid.Enumerate(dongleInfo.VendorID, dongleInfo.ProductID, func(info *hid.DeviceInfo) error {
		if info.UsagePage == dongleAUsagePage {
			currentRoot := usb.GetHardwareID(info.Path)
			if currentRoot == targetRoot {
				enumaretedDongleInfo = *info
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return mo.Err[int](err)
	}

	dongle, err := hid.OpenPath(enumaretedDongleInfo.Path)
	if err != nil {
		return mo.Err[int](err)
	}
	defer func(device hid.Device) {
		err := device.Close()
		if err != nil {
			slog.Error("Failed to close dongle", "device", device)
		}
	}(*dongle)

	buffer := make([]byte, 33)
	buffer[1] = 0xb2
	_, err = dongle.SendOutputReport(buffer)
	if err != nil {
		return mo.Err[int](err)
	}

	_, err = dongle.ReadWithTimeout(buffer, 1*time.Second)
	if err != hid.ErrTimeout && err != nil {
		return mo.Err[int](err)
	}

	if buffer[0] == 0xb2 && buffer[12] == 0x34 && buffer[13] == 0x34 {
		return mo.Ok(int(binary.LittleEndian.Uint16(buffer[14:16])))
	} else {
		return mo.Err[int](ErrInvalidResponseFromDongle)
	}
}

func getDongleDevice(hidDevice hid.DeviceInfo) int {
	productID := 0
	if slices.Contains(dongleProductIDA, hidDevice.ProductID) {
		maybeProductID := getProductIDOfDongleA(hidDevice)
		if maybeProductID.IsOk() {
			productID = maybeProductID.MustGet()
		} else {
			slog.Error("Failed to get device productID from dongle", "productID", fmt.Sprintf("0x%x", hidDevice.ProductID))
		}
	} else if slices.Contains(dongleProductIDB, hidDevice.ProductID) {
		maybeProductID := getProductIDOfDongleB(hidDevice)
		if maybeProductID.IsOk() {
			productID = maybeProductID.MustGet()
		} else {
			slog.Error("Failed to get device productID from dongle", "productID", fmt.Sprintf("0x%x", hidDevice.ProductID))
		}
	}

	switch productID {
	case keychronM3.PRODUCT_ID_DEVICE:
		return KeychronM3Dongle
	case keychronM6.PRODUCT_ID_DEVICE:
		return KeychronM6Dongle
	default:
		return Unknown
	}
}
