package device

import (
	"context"

	"github.com/samber/mo"
)

type Driver interface {
	BatteryPercentage() mo.Option[int]
	SubscribeBatteryPercentage() chan int
	UnsubscribeBatteryPercentage(chan int)
	Init(ctx context.Context) error
	StartBackgroundCheck(ctx context.Context)
}
