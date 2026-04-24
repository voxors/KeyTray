package keychronM3

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

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
	dongleMutex             sync.Mutex
	deviceMutex             sync.Mutex
	currentPourcentage      mo.Option[int]
	currentPourcentageMutex sync.RWMutex
}

var (
	ErrInvalidDevice  = errors.New("invalid device")
	ErrDataNotFound   = errors.New("data not found")
	ErrDeviceNotFound = errors.New("device not found")
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

	return mo.Ok(&m3MouseBattery{
		dongleMutex:             sync.Mutex{},
		deviceMutex:             sync.Mutex{},
		currentPourcentage:      mo.None[int](),
		currentPourcentageMutex: sync.RWMutex{},
	})
}

func (m *m3MouseBattery) setCurrentPourcentage(pourcentage int) {
	m.currentPourcentageMutex.Lock()
	defer m.currentPourcentageMutex.Unlock()
	m.currentPourcentage = mo.Some(pourcentage)
}

func (m *m3MouseBattery) getCurrentPourcentage() mo.Option[int] {
	m.currentPourcentageMutex.RLock()
	defer m.currentPourcentageMutex.RUnlock()
	return m.currentPourcentage
}

func (m *m3MouseBattery) startWorkers() {
	go m.workerPourcentageInteruptListener(PRODUCT_ID_DEVICE)
	go m.workerPourcentageInteruptListener(PRODUCT_ID_DONGLE)
}

func (m *m3MouseBattery) workerPourcentageInteruptListener(productID int) {
	deviceNotFoundWarmed := false
	for {
		var deviceInfo *hid.DeviceInfo
		hid.Enumerate(VENDOR_ID, hid.ProductIDAny, func(info *hid.DeviceInfo) error {
			if deviceInfo == nil &&
				CheckHidInfoValid(info) &&
				info.ProductID == uint16(productID) {
				deviceInfo = info
			}
			return nil
		})

		if deviceInfo == nil {
			if !deviceNotFoundWarmed {
				slog.Warn(
					"Device not found",
					"Vendor ID", fmt.Sprintf("0x%x", VENDOR_ID),
					"Product ID", fmt.Sprintf("0x%x", productID),
				)
				deviceNotFoundWarmed = true
			}
			time.Sleep(5 * time.Second)
			continue
		}
		deviceNotFoundWarmed = false

		var mutex *sync.Mutex
		switch deviceInfo.ProductID {
		case PRODUCT_ID_DONGLE:
			mutex = &m.dongleMutex
		case PRODUCT_ID_DEVICE:
			mutex = &m.deviceMutex
		}
		if mutex != nil {
			mutex.Lock()
		}
		device, err := hid.OpenPath(deviceInfo.Path)
		if err != nil {
			slog.Error("Failed to open device", "device info", deviceInfo)
			continue
		}

		for {
			buffer := make([]byte, 64)
			readBytes, err := device.Read(buffer)
			if err != nil {
				slog.Error("Failed to read", "error", err)
				break
			}

			if readBytes > 4 {
				slog.Debug(
					"Keychron m3 Mouse buffer",
					"productID", fmt.Sprintf("0x%x", deviceInfo.ProductID),
					"buffer", fmt.Sprintf("% x", buffer),
				)

				if buffer[1] == 0xE2 {
					m.setCurrentPourcentage(int(buffer[5]))
				}
			}
		}

		device.Close()
		mutex.Unlock()
	}
}

func (m *m3MouseBattery) getPourcentageThroughtFeatureReport() mo.Result[int] {
	var deviceInfo *hid.DeviceInfo
	hid.Enumerate(VENDOR_ID, hid.ProductIDAny, func(info *hid.DeviceInfo) error {
		if deviceInfo == nil && CheckHidInfoValid(info) {
			deviceInfo = info
		}
		return nil
	})

	if deviceInfo == nil {
		return mo.Err[int](ErrDeviceNotFound)
	}

	var mutex *sync.Mutex
	switch deviceInfo.ProductID {
	case PRODUCT_ID_DONGLE:
		mutex = &m.dongleMutex
	case PRODUCT_ID_DEVICE:
		mutex = &m.deviceMutex
	}
	if mutex != nil {
		mutex.Lock()
		defer mutex.Unlock()
	}

	device, err := hid.OpenPath(deviceInfo.Path)
	if err != nil {
		return mo.Err[int](err)
	}
	defer device.Close()

	buffer := make([]byte, 64)

	var lasterr error
	var retries int
	for retries = range 5 {
		buffer[0] = 0x51
		readlen, err := device.GetFeatureReport(buffer)
		slog.Debug(
			"Keychron m3 Mouse buffer",
			"productID", fmt.Sprintf("%x", deviceInfo.ProductID),
			"buffer", fmt.Sprintf("% x", buffer),
		)
		if err != nil {
			lasterr = err
			continue
		}

		if readlen < 12 {
			lasterr = ErrDataNotFound
			continue
		}

		break
	}

	if retries >= 5 {
		return mo.Err[int](lasterr)
	}

	pourcentage := int(buffer[11])
	m.setCurrentPourcentage(pourcentage)
	return mo.Ok(pourcentage)
}

func (m *m3MouseBattery) Pourcentage() mo.Result[int] {
	pourcentage, presence := m.getCurrentPourcentage().Get()
	if !presence {
		result := m.getPourcentageThroughtFeatureReport()
		m.startWorkers()
		return result
	}

	return mo.Ok(pourcentage)
}
