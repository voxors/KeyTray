package keychronM3

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/samber/mo"
	"github.com/sstallion/go-hid"
	"github.com/voxors/KeyTray/src/pkg/broadcast"
)

const (
	VENDOR_ID          = 0x3434
	PRODUCT_ID_DONGLE  = 0xD030
	PRODUCT_ID_DEVICE  = 0xD033
	BATTERY_USAGE_PAGE = 140
)

type keychronM3Info struct {
	dongleMutex            sync.Mutex
	deviceMutex            sync.Mutex
	batteryPercentage      mo.Option[int]
	batteryPercentageMutex sync.RWMutex
	batteryBroadcast       broadcast.Broadcast[int]
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

func NewKeychronM3Driver(hidDevice *hid.DeviceInfo) mo.Result[*keychronM3Info] {
	if hidDevice.VendorID != VENDOR_ID ||
		hidDevice.ProductID != PRODUCT_ID_DONGLE &&
			hidDevice.ProductID != PRODUCT_ID_DEVICE {
		return mo.Err[*keychronM3Info](ErrInvalidDevice)
	}

	return mo.Ok(&keychronM3Info{
		dongleMutex:            sync.Mutex{},
		deviceMutex:            sync.Mutex{},
		batteryPercentage:      mo.None[int](),
		batteryPercentageMutex: sync.RWMutex{},
		batteryBroadcast:       broadcast.NewBroadcast[int](),
	})
}

func (k *keychronM3Info) setCurrentPercentage(percentage int) {
	k.batteryPercentageMutex.Lock()
	defer k.batteryPercentageMutex.Unlock()
	k.batteryPercentage = mo.Some(percentage)
	k.batteryBroadcast.Send(percentage)
}

func (k *keychronM3Info) getCurrentPercentage() mo.Option[int] {
	k.batteryPercentageMutex.RLock()
	defer k.batteryPercentageMutex.RUnlock()
	return k.batteryPercentage
}

func (k *keychronM3Info) StartBackgroundCheck(ctx context.Context) {
	go k.workerPercentageInteruptListener(ctx, PRODUCT_ID_DEVICE)
	go k.workerPercentageInteruptListener(ctx, PRODUCT_ID_DONGLE)
}

func (k *keychronM3Info) workerPercentageInteruptListener(ctx context.Context, productID int) {
	deviceNotFoundWarmed := false
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

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
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
		deviceNotFoundWarmed = false

		var mutex *sync.Mutex
		switch deviceInfo.ProductID {
		case PRODUCT_ID_DONGLE:
			mutex = &k.dongleMutex
		case PRODUCT_ID_DEVICE:
			mutex = &k.deviceMutex
		}
		if mutex != nil {
			mutex.Lock()
		}
		device, err := hid.OpenPath(deviceInfo.Path)
		if err != nil {
			slog.Error("Failed to open device", "device info", deviceInfo)
			continue
		}

		connectionCtx, connCancel := context.WithCancel(ctx)
		go func(device *hid.Device, context context.Context) {
			<-context.Done()
			device.Close()
		}(device, connectionCtx)

	ReadLoopLabel:
		for {
			select {
			case <-connectionCtx.Done():
				break ReadLoopLabel
			default:
			}

			buffer := make([]byte, 64)
			readBytes, err := device.Read(buffer)
			if err != nil {
				slog.Error("Failed to read", "error", err)
				break
			}

			if readBytes > 4 {
				slog.Debug(
					"Keychron M3 Mouse buffer",
					"productID", fmt.Sprintf("0x%x", deviceInfo.ProductID),
					"buffer", fmt.Sprintf("% x", buffer),
				)

				if buffer[1] == 0xE2 {
					k.setCurrentPercentage(int(buffer[5]))
				}
			}
		}

		connCancel()
		if mutex != nil {
			mutex.Unlock()
		}
	}
}

func (k *keychronM3Info) getPercentageThroughtFeatureReport(ctx context.Context) mo.Result[int] {
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
		mutex = &k.dongleMutex
	case PRODUCT_ID_DEVICE:
		mutex = &k.deviceMutex
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
		select {
		case <-ctx.Done():
			return mo.Err[int](ctx.Err())
		default:
		}

		buffer[0] = 0x51
		readlen, err := device.GetFeatureReport(buffer)
		slog.Debug(
			"Keychron M3 Mouse buffer",
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

	percentage := int(buffer[11])
	k.setCurrentPercentage(percentage)
	return mo.Ok(percentage)
}

func (k *keychronM3Info) Init(ctx context.Context) error {
	_, err := k.getPercentageThroughtFeatureReport(ctx).Get()
	return err
}

func (k *keychronM3Info) BatteryPercentage() mo.Option[int] {
	return k.getCurrentPercentage()
}

func (k *keychronM3Info) SubscribeBatteryPercentage() chan int {
	return k.batteryBroadcast.AddListener()
}

func (k *keychronM3Info) UnsubscribeBatteryPercentage(channel chan int) {
	k.batteryBroadcast.RemoveListener(channel)
}
