package m3Mouse

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/samber/mo"
	"github.com/sstallion/go-hid"
)

const (
	VENDOR_ID          = 0x3434
	PRODUCT_ID_DONGLE  = 0xD030
	PRODUCT_ID_DEVICE  = 0xD033
	BATTERY_USAGE_PAGE = 140
)

type m3MouseBattery struct {
}

var (
	ErrInvalidDevice = errors.New("invalid device")
	ErrDataNotFound  = errors.New("data not found")
)

func CheckHidInfoValid(hidDevice *hid.DeviceInfo) bool {
	if hidDevice.VendorID == VENDOR_ID &&
		hidDevice.UsagePage == BATTERY_USAGE_PAGE &&
		(hidDevice.ProductID == PRODUCT_ID_DEVICE ||
			hidDevice.ProductID == PRODUCT_ID_DONGLE) {
		return true
	}
	return false
}

func NewM3MouseBattery(hidDevice *hid.DeviceInfo) mo.Result[*m3MouseBattery] {
	if hidDevice.VendorID != VENDOR_ID ||
		hidDevice.ProductID != PRODUCT_ID_DONGLE &&
			hidDevice.ProductID != PRODUCT_ID_DEVICE {
		return mo.Err[*m3MouseBattery](ErrInvalidDevice)
	}

	return mo.Ok(&m3MouseBattery{})
}

func (m m3MouseBattery) Pourcentage() mo.Result[int] {
	var deviceInfo hid.DeviceInfo
	hid.Enumerate(hid.VendorIDAny, hid.ProductIDAny, func(info *hid.DeviceInfo) error {
		if CheckHidInfoValid(info) {
			if deviceInfo.ProductID != PRODUCT_ID_DEVICE {
				deviceInfo = *info
			}
		}
		return nil
	})

	slog.Info("wtf", "productID", fmt.Sprintf("0x%X", deviceInfo.ProductID))
	device, err := hid.OpenPath(deviceInfo.Path)
	if err != nil {
		return mo.Err[int](err)
	}
	defer device.Close()

	buffer := make([]byte, 64)
	buffer[0] = 0x51
	for {
		readlen, err := device.GetFeatureReport(buffer)
		if err == hid.ErrTimeout {
			continue
		}
		if err != nil {
			return mo.Err[int](err)
		}

		if readlen < 12 {
			return mo.Err[int](ErrDataNotFound)
		}

		break
	}

	return mo.Ok((int)(buffer[11]))
}
