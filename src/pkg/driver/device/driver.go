package device

import (
	"context"

	"github.com/samber/mo"
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
}
