package keychronM6

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
	"github.com/voxors/KeyTray/src/pkg/driver/dtype"
)

const (
	VENDOR_ID                 = 0x3434
	PRODUCT_ID_DONGLE_USB_A   = 0xD026
	PRODUCT_ID_DONGLE_USB_C   = 0xD029
	PRODUCT_ID_DEVICE         = 0xD060
	BATTERY_USAGE_PAGE_DONGLE = 0x8C
	BATTERY_USAGE_PAGE_DEVICE = 0xFFC1
)

type keychronM6Info struct {
	dongleMutex            sync.Mutex
	deviceMutex            sync.Mutex
	dongleInfo             mo.Option[hid.DeviceInfo]
	deviceInfo             mo.Option[hid.DeviceInfo]
	batteryPercentage      mo.Option[int]
	batteryPercentageMutex sync.RWMutex
	batteryBroadcast       broadcast.Broadcast[int]
	isCharging             mo.Option[bool]
	isChargingMutex        sync.RWMutex
	isChargingBroadcast    broadcast.Broadcast[bool]
	cancelContextDongle    mo.Option[context.CancelFunc]
	cancelContextDevice    mo.Option[context.CancelFunc]
	backgroundWatchStarted bool
}

var (
	ErrInvalidDevice        = errors.New("invalid device")
	ErrDataNotFound         = errors.New("data not found")
	ErrDeviceNotFound       = errors.New("device not found")
	ErrInvalidResponse      = errors.New("invalid response")
	ErrInvalidDeviceFeature = errors.New("invalid device for feature request")
	ErrNoDevice             = errors.New("no device set in the driver")
)

func CheckHidInfoValid(hidDevice hid.DeviceInfo) bool {
	if hidDevice.VendorID == VENDOR_ID &&
		(hidDevice.ProductID == PRODUCT_ID_DEVICE && hidDevice.UsagePage == BATTERY_USAGE_PAGE_DEVICE) ||
		((hidDevice.ProductID == PRODUCT_ID_DONGLE_USB_A ||
			hidDevice.ProductID == PRODUCT_ID_DONGLE_USB_C) &&
			hidDevice.UsagePage == BATTERY_USAGE_PAGE_DONGLE) {
		return true
	}
	return false
}

func NewKeychronM6Driver() *keychronM6Info {
	return &keychronM6Info{
		dongleMutex:            sync.Mutex{},
		deviceMutex:            sync.Mutex{},
		batteryPercentage:      mo.None[int](),
		batteryPercentageMutex: sync.RWMutex{},
		batteryBroadcast:       broadcast.NewBroadcast[int](),
		isCharging:             mo.Option[bool]{},
		isChargingMutex:        sync.RWMutex{},
		isChargingBroadcast:    broadcast.NewBroadcast[bool](),
		dongleInfo:             mo.None[hid.DeviceInfo](),
		deviceInfo:             mo.None[hid.DeviceInfo](),
		cancelContextDongle:    mo.None[context.CancelFunc](),
		cancelContextDevice:    mo.None[context.CancelFunc](),
		backgroundWatchStarted: false,
	}
}

func (k *keychronM6Info) setCurrentPercentage(percentage int) {
	k.batteryPercentageMutex.Lock()
	defer k.batteryPercentageMutex.Unlock()
	k.batteryPercentage = mo.Some(percentage)
	k.batteryBroadcast.Send(percentage)
}

func (k *keychronM6Info) getCurrentPercentage() mo.Option[int] {
	k.batteryPercentageMutex.RLock()
	defer k.batteryPercentageMutex.RUnlock()
	return k.batteryPercentage
}

func (k *keychronM6Info) getIsCharging() mo.Option[bool] {
	k.isChargingMutex.Lock()
	defer k.isChargingMutex.Unlock()
	return k.isCharging
}

func (k *keychronM6Info) setIsCharging(isCharging bool) {
	k.isChargingMutex.Lock()
	defer k.isChargingMutex.Unlock()
	k.isCharging = mo.Some(isCharging)
	k.isChargingBroadcast.Send(isCharging)
}

func (k *keychronM6Info) StartBackgroundCheck(ctx context.Context) {
	if k.deviceInfo.IsSome() {
		k.startDeviceWorker(ctx)
	}
	if k.dongleInfo.IsSome() {
		k.startDongleWorker(ctx)
	}
	k.backgroundWatchStarted = true
}

func (k *keychronM6Info) startDongleWorker(ctx context.Context) {
	if k.cancelContextDongle.IsSome() {
		k.cancelContextDongle.MustGet()()
	}
	if k.dongleInfo.IsNone() {
		slog.Error("Failed to start dongle worker")
	} else {
		go k.workerPercentageInteruptListener(ctx, k.dongleInfo.MustGet())
	}
}

func (k *keychronM6Info) startDeviceWorker(ctx context.Context) {
	if k.cancelContextDevice.IsSome() {
		k.cancelContextDevice.MustGet()()
	}
	if k.deviceInfo.IsNone() {
		slog.Error("Failed to start device worker")
	} else {
		go k.workerPercentageInteruptListener(ctx, k.deviceInfo.MustGet())
	}
}

func (k *keychronM6Info) StopBackgroundCheck() {
	if k.cancelContextDevice.IsSome() {
		k.cancelContextDevice.MustGet()()
	}
	if k.cancelContextDongle.IsSome() {
		k.cancelContextDongle.MustGet()()
	}
	k.backgroundWatchStarted = false
}

func (k *keychronM6Info) workerPercentageInteruptListener(ctx context.Context, deviceInfo hid.DeviceInfo) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var mutex *sync.Mutex
		switch deviceInfo.ProductID {
		case PRODUCT_ID_DONGLE_USB_A:
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
				slog.Warn("Failed to close device", "Product ID", fmt.Sprintf("0x%x", deviceInfo.ProductID))
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
					"Keychron M6 Mouse buffer",
					"productID", fmt.Sprintf("0x%x", deviceInfo.ProductID),
					"buffer", fmt.Sprintf("% x", buffer),
				)

				if buffer[1] == 0xE2 {
					if buffer[5] != 0xFF {
						k.setCurrentPercentage(int(buffer[5]))
					}
					if buffer[4] != 0xFF {
						k.setIsCharging(buffer[4] != 0)
					}
				}
			}
		}

		connCancel()
		if mutex != nil {
			mutex.Unlock()
		}
	}
}

func (k *keychronM6Info) getPercentageThroughtFeatureReport(ctx context.Context) mo.Result[int] {
	var deviceInfo hid.DeviceInfo
	if k.deviceInfo.IsSome() {
		deviceInfo = k.deviceInfo.MustGet()
	} else if k.dongleInfo.IsSome() {
		deviceInfo = k.dongleInfo.MustGet()
	} else {
		return mo.Err[int](ErrNoDevice)
	}

	var mutex *sync.Mutex
	switch deviceInfo.ProductID {
	case PRODUCT_ID_DONGLE_USB_A:
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

	buffer := make([]byte, 128)

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

		percentage := int(127 & buffer[20])
		powerState := int(buffer[20] >> 7)
		k.setCurrentPercentage(percentage)
		k.setIsCharging(powerState != 0)
		return mo.Ok(percentage)
	}

	return mo.Err[int](lasterr)
}

func (*keychronM6Info) getfeatureReport(buffer []byte, device *hid.Device, deviceInfo hid.DeviceInfo) error {
	payload := make([]byte, 64)
	payload[0] = 0xb3
	payload[1] = 0x06
	switch deviceInfo.ProductID {
	case PRODUCT_ID_DEVICE:
		_, err := device.SendOutputReport(payload)
		if err != nil {
			return err
		}
	case PRODUCT_ID_DONGLE_USB_A:
		fallthrough
	case PRODUCT_ID_DONGLE_USB_C:
		_, err := device.Write(payload)
		if err != nil {
			return err
		}
	default:
		return ErrInvalidDeviceFeature
	}

	readlen, err := device.Read(buffer)
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

func (k *keychronM6Info) Init(ctx context.Context) error {
	_, err := k.getPercentageThroughtFeatureReport(ctx).Get()
	return err
}

func (k *keychronM6Info) BatteryPercentage() mo.Option[int] {
	return k.getCurrentPercentage()
}

func (k *keychronM6Info) GetIsCharging() mo.Option[bool] {
	return k.getIsCharging()
}

func (k *keychronM6Info) SubscribeBatteryPercentage() chan int {
	return k.batteryBroadcast.AddListener()
}

func (k *keychronM6Info) UnsubscribeBatteryPercentage(channel chan int) {
	k.batteryBroadcast.RemoveListener(channel)
}

func (k *keychronM6Info) SubscribeIsCharging() chan bool {
	return k.isChargingBroadcast.AddListener()
}

func (k *keychronM6Info) UnsubscribeIsChargin(channel chan bool) {
	k.isChargingBroadcast.RemoveListener(channel)
}

func (k *keychronM6Info) GetProductID() []int {
	return []int{PRODUCT_ID_DEVICE, PRODUCT_ID_DONGLE_USB_A}
}

func (k *keychronM6Info) GetVendorID() []int {
	return []int{VENDOR_ID}
}

func (k *keychronM6Info) GetDeviceName() string {
	return "Keychron M6"
}

func (k *keychronM6Info) GetDeviceInfo() []hid.DeviceInfo {
	deviceList := make([]hid.DeviceInfo, 0)
	if k.deviceInfo.IsSome() {
		deviceList = append(deviceList, k.deviceInfo.MustGet())
	}
	if k.dongleInfo.IsSome() {
		deviceList = append(deviceList, k.dongleInfo.MustGet())
	}
	return deviceList
}

func (k *keychronM6Info) AddDeviceInfo(ctx context.Context, info hid.DeviceInfo) error {
	switch info.ProductID {
	case PRODUCT_ID_DEVICE:
		k.deviceInfo = mo.Some(info)
		if k.backgroundWatchStarted {
			k.startDeviceWorker(ctx)
		}
	case PRODUCT_ID_DONGLE_USB_A:
		fallthrough
	case PRODUCT_ID_DONGLE_USB_C:
		k.dongleInfo = mo.Some(info)
		if k.backgroundWatchStarted {
			k.startDongleWorker(ctx)
		}
	default:
		return ErrInvalidDevice
	}

	return nil
}

func (k *keychronM6Info) RemoveDeviceInfo(info hid.DeviceInfo) {
	if k.dongleInfo.IsSome() && k.dongleInfo.MustGet() == info {
		if k.cancelContextDongle.IsSome() {
			k.cancelContextDongle.MustGet()()
			k.cancelContextDongle = mo.None[context.CancelFunc]()
		}
		k.dongleInfo = mo.None[hid.DeviceInfo]()
	}
	if k.deviceInfo.IsSome() && k.deviceInfo.MustGet() == info {
		if k.cancelContextDevice.IsSome() {
			k.cancelContextDevice.MustGet()()
			k.cancelContextDevice = mo.None[context.CancelFunc]()
		}
		k.deviceInfo = mo.None[hid.DeviceInfo]()
	}
}

func (k *keychronM6Info) GetDriverType() int {
	return dtype.KeychronM6
}
