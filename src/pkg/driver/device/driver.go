package device

import (
	"context"

	"github.com/samber/mo"
	"github.com/sstallion/go-hid"
)

type Driver interface {
	BatteryPercentage() mo.Option[int]
	GetIsCharging() mo.Option[bool]
	SubscribeBatteryPercentage() chan int
	UnsubscribeBatteryPercentage(chan int)
	SubscribeIsCharging() chan bool
	UnsubscribeIsChargin(chan bool)
	Init(ctx context.Context) error
	StartBackgroundCheck(ctx context.Context)
	StopBackgroundCheck()
	GetProductID() []int
	GetVendorID() []int
	GetDeviceName() string
	GetDeviceInfo() []hid.DeviceInfo
	AddDeviceInfo(context.Context, hid.DeviceInfo) error
	RemoveDeviceInfo(hid.DeviceInfo)
	GetDriverType() int
}
