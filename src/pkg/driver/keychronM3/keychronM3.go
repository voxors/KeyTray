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
	isCharging             mo.Option[bool]
	isChargingMutex        sync.RWMutex
	isChargingBroadcast    broadcast.Broadcast[bool]
	cancelContext          mo.Option[context.CancelFunc]
}

var (
	ErrInvalidDevice   = errors.New("invalid device")
	ErrDataNotFound    = errors.New("data not found")
	ErrDeviceNotFound  = errors.New("device not found")
	ErrInvalidResponse = errors.New("invalid response")
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
		isCharging:             mo.Option[bool]{},
		isChargingMutex:        sync.RWMutex{},
		isChargingBroadcast:    broadcast.NewBroadcast[bool](),
		cancelContext:          mo.None[context.CancelFunc](),
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

func (k *keychronM3Info) getIsCharging() mo.Option[bool] {
	k.isChargingMutex.Lock()
	defer k.isChargingMutex.Unlock()
	return k.isCharging
}

func (k *keychronM3Info) setIsCharging(isCharging bool) {
	k.isChargingMutex.Lock()
	defer k.isChargingMutex.Unlock()
	k.isCharging = mo.Some(isCharging)
	k.isChargingBroadcast.Send(isCharging)
}

func (k *keychronM3Info) StartBackgroundCheck(ctx context.Context) {
	if k.cancelContext.IsSome() {
		k.cancelContext.MustGet()()
	}
	cancelctx, cancel := context.WithCancel(ctx)
	k.cancelContext = mo.Some(cancel)
	go k.workerPercentageInteruptListener(cancelctx, PRODUCT_ID_DEVICE)
	go k.workerPercentageInteruptListener(cancelctx, PRODUCT_ID_DONGLE)
}

func (k *keychronM3Info) StopBackgroundCheck() {
	if k.cancelContext.IsSome() {
		k.cancelContext.MustGet()()
	}
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
		err := hid.Enumerate(VENDOR_ID, hid.ProductIDAny, func(info *hid.DeviceInfo) error {
			if deviceInfo == nil &&
				CheckHidInfoValid(info) &&
				info.ProductID == uint16(productID) {
				deviceInfo = info
			}
			return nil
		})
		if err != nil {
			slog.Error("Failed to enumerate devices", "error", err)
		}

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
			err := device.Close()
			if err != nil {
				slog.Warn("Failed to close device", "Product ID", fmt.Sprintf("0x%x", productID))
			}
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
					k.setIsCharging(buffer[4] != 0)
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
	err := hid.Enumerate(VENDOR_ID, hid.ProductIDAny, func(info *hid.DeviceInfo) error {
		if deviceInfo == nil && CheckHidInfoValid(info) {
			deviceInfo = info
		}
		return nil
	})
	if err != nil {
		return mo.Err[int](err)
	}

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
	defer func(device *hid.Device) {
		if device != nil {
			err := device.Close()
			if err != nil {
				slog.Warn("Failed to close device", "device", device)
			}
		}
	}(device)

	buffer := make([]byte, 64)

	var lasterr error
	const maxRetries = 6
	for range maxRetries {
		select {
		case <-ctx.Done():
			return mo.Err[int](ctx.Err())
		default:
		}

		err = k.getfeatureReport(buffer, device, deviceInfo)
		if err != nil {
			lasterr = err
			select {
			case <-ctx.Done():
				return mo.Err[int](ctx.Err())
			default:
				time.Sleep(1 * time.Second)
				continue
			}
		}

		percentage := int(buffer[11])
		powerState := int(3 & buffer[12])
		k.setCurrentPercentage(percentage)
		k.setIsCharging(powerState != 0)
		return mo.Ok(percentage)
	}

	return mo.Err[int](lasterr)
}

func (*keychronM3Info) getfeatureReport(buffer []byte, device *hid.Device, deviceInfo *hid.DeviceInfo) error {
	buffer[0] = 0x51
	readlen, err := device.GetFeatureReport(buffer)
	slog.Debug(
		"Keychron M3 Mouse buffer",
		"productID", fmt.Sprintf("%x", deviceInfo.ProductID),
		"buffer", fmt.Sprintf("% x", buffer),
	)
	if err != nil {
		return err
	}

	isAllZero := true
	for _, byte := range buffer[1:] {
		if byte != 0 {
			isAllZero = false
			break
		}
	}

	if isAllZero {
		return ErrInvalidResponse
	}

	if readlen < 12 {
		return ErrDataNotFound
	}

	return nil
}

func (k *keychronM3Info) Init(ctx context.Context) error {
	_, err := k.getPercentageThroughtFeatureReport(ctx).Get()
	return err
}

func (k *keychronM3Info) BatteryPercentage() mo.Option[int] {
	return k.getCurrentPercentage()
}

func (k *keychronM3Info) GetIsCharging() mo.Option[bool] {
	return k.getIsCharging()
}

func (k *keychronM3Info) SubscribeBatteryPercentage() chan int {
	return k.batteryBroadcast.AddListener()
}

func (k *keychronM3Info) UnsubscribeBatteryPercentage(channel chan int) {
	k.batteryBroadcast.RemoveListener(channel)
}

func (k *keychronM3Info) SubscribeIsCharging() chan bool {
	return k.isChargingBroadcast.AddListener()
}

func (k *keychronM3Info) UnsubscribeIsChargin(channel chan bool) {
	k.isChargingBroadcast.RemoveListener(channel)
}

func (k *keychronM3Info) GetProductID() []int {
	return []int{PRODUCT_ID_DEVICE, PRODUCT_ID_DONGLE}
}

func (k *keychronM3Info) GetVendorID() []int {
	return []int{VENDOR_ID}
}

func (k *keychronM3Info) GetDeviceName() string {
	return "Keychron M3"
}
