package commoninterface

import (
	"context"

	"github.com/samber/mo"
)

type Driver interface {
	BatteryPercentage() mo.Option[int]
	Init(ctx context.Context) error
	StartBackgroundCheck(ctx context.Context)
}
