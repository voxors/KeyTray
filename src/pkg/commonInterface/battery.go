package commoninterface

import "github.com/samber/mo"

type BatteryInfo interface {
	Pourcentage() mo.Result[int]
}
