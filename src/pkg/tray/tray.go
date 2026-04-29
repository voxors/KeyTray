package tray

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"slices"
	"strings"

	"deedles.dev/tray"
	"github.com/samber/mo"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"github.com/voxors/KeyTray/src/pkg/driver/device"
)

const (
	logoSizePixmap = 128
)

type Keytray struct {
	item              *tray.Item
	driver            []device.Driver
	logo              mo.Option[*image.RGBA]
	watcherCancelFunc mo.Option[context.CancelFunc]
	quitItem          mo.Option[*tray.MenuItem]
	separatorItem     mo.Option[*tray.MenuItem]
	batteryItems      map[device.Driver]*tray.MenuItem
}

func Init() mo.Result[Keytray] {
	item, err := tray.New(
		tray.ItemID("Keytray"),
		tray.ItemTitle("Keytray"),
		tray.ItemIsMenu(true),
	)
	if err != nil {
		return mo.Err[Keytray](err)
	}

	keytray := Keytray{
		item:              item,
		driver:            []device.Driver{},
		logo:              mo.None[*image.RGBA](),
		watcherCancelFunc: mo.None[context.CancelFunc](),
		quitItem:          mo.None[*tray.MenuItem](),
		separatorItem:     mo.None[*tray.MenuItem](),
		batteryItems:      make(map[device.Driver]*tray.MenuItem),
	}

	return mo.Ok(keytray)
}

func (k *Keytray) StartDeviceWatcher(ctx context.Context) {
	if k.watcherCancelFunc.IsSome() {
		k.watcherCancelFunc.MustGet()()
	}
	watcherCtx, watcherCancelFunc := context.WithCancel(ctx)
	k.watcherCancelFunc = mo.Some(watcherCancelFunc)
	for _, driver := range k.driver {
		batteryUpdatesChan := driver.SubscribeBatteryPercentage()
		isChargingUpdatesChan := driver.SubscribeIsCharging()
		go func(d device.Driver, batteryUpdateChan chan int, isChargingUpdateChan chan bool) {
			for {
				select {
				case <-watcherCtx.Done():
					d.UnsubscribeBatteryPercentage(batteryUpdateChan)
					d.UnsubscribeIsChargin(isChargingUpdatesChan)
					return
				case <-batteryUpdateChan:
					err := k.updateTray()
					if err != nil {
						slog.Warn("Failed to update tray", "error", err)
					}
				case <-isChargingUpdateChan:
					err := k.updateTray()
					if err != nil {
						slog.Warn("Failed to update tray", "error", err)
					}
				}
			}
		}(driver, batteryUpdatesChan, isChargingUpdatesChan)
	}
}

func (k *Keytray) updateTray() error {
	var tooltipTexts []string
	lowestPercentage := mo.None[int]()
	lowestIsCharging := mo.None[bool]()
	for _, driver := range k.driver {
		percentage, percentageExist := driver.BatteryPercentage().Get()
		isCharging, isChargingExist := driver.GetIsCharging().Get()
		if percentageExist {
			tooltipText := fmt.Sprintf("%s: %d%%", driver.GetDeviceName(), percentage)
			if lowestPercentage.IsNone() {
				lowestPercentage = mo.Some(percentage)
				if isChargingExist {
					lowestIsCharging = mo.Some(isCharging)
				}
			} else if percentage < lowestPercentage.MustGet() {
				lowestPercentage = mo.Some(percentage)
				if isChargingExist {
					lowestIsCharging = mo.Some(isCharging)
				}
			}
			tooltipTexts = append(tooltipTexts, tooltipText)
		} else {
			tooltipTexts = append(tooltipTexts, fmt.Sprintf("%s: %s", driver.GetDeviceName(), "Unavailable"))
		}

		var iconName string
		tooltipText := tooltipTexts[len(tooltipTexts)-1]
		if percentageExist {
			iconNamePercentage := (percentage / 10) * 10
			iconName = fmt.Sprintf("battery-%03d", iconNamePercentage)
			if isChargingExist && isCharging {
				iconName = fmt.Sprintf("%s%s", iconName, "-charging")
			}
		}
		if batteryMenuItem, ok := k.batteryItems[driver]; ok {
			err := batteryMenuItem.SetProps(
				tray.MenuItemLabel(tooltipText),
				tray.MenuItemIconName(iconName),
			)
			if err != nil {
				return err
			}
		} else {
			batteryMenuItem, err := k.item.Menu().AddChild(
				tray.MenuItemLabel(tooltipText),
				tray.MenuItemIconName(iconName),
			)
			if err != nil {
				return err
			}
			k.batteryItems[driver] = batteryMenuItem
		}
	}

	newBatteryMenuItemsMap := make(map[device.Driver]*tray.MenuItem)
	for driver, batteryMenuItem := range k.batteryItems {
		if slices.Contains(k.driver, driver) {
			newBatteryMenuItemsMap[driver] = batteryMenuItem
		} else {
			err := batteryMenuItem.Remove()
			if err != nil {
				return err
			}
		}
	}
	k.batteryItems = newBatteryMenuItemsMap

	if k.quitItem.IsSome() && len(k.batteryItems) != 0 {
		if k.separatorItem.IsNone() {
			separatorItem, err := k.item.Menu().AddChild(
				tray.MenuItemType(tray.Separator),
			)
			if err != nil {
				return err
			}
			k.separatorItem = mo.Some(separatorItem)
		}
		err := k.item.Menu().AppendChild(k.separatorItem.MustGet())
		if err != nil {
			return err
		}
		err = k.item.Menu().AppendChild(k.quitItem.MustGet())
		if err != nil {
			return err
		}
	} else if k.separatorItem.IsSome() {
		err := k.separatorItem.MustGet().Remove()
		if err != nil {
			return err
		}
		k.separatorItem = mo.None[*tray.MenuItem]()
	}

	var iconName string
	if lowestPercentage.IsSome() {
		iconNamePercentage := (lowestPercentage.MustGet() / 10) * 10
		iconName = fmt.Sprintf("battery-%03d", iconNamePercentage)
		if lowestIsCharging.IsSome() && lowestIsCharging.MustGet() {
			iconName = fmt.Sprintf("%s%s", iconName, "-charging")
		}
	} else {
		if k.logo.IsSome() {
			err := k.item.SetProps(
				tray.ItemIconPixmap(k.logo.MustGet()),
			)
			if err != nil {
				slog.Warn("Failed to set logo", "error", err)
			}
		}
	}
	err := k.item.SetProps(
		tray.ItemToolTip("", nil, "Keytray", strings.Join(tooltipTexts, "\n")),
		tray.ItemIconName(iconName),
	)
	if err != nil {
		return err
	}

	return nil
}

func (k *Keytray) SetDevices(ctx context.Context, devices []device.Driver) error {
	k.driver = devices
	err := k.updateTray()
	if err != nil {
		return err
	}
	k.StartDeviceWatcher(ctx)
	return nil
}

func (k *Keytray) SetLogo(svgContent string) error {
	svgIcon, err := oksvg.ReadIconStream(strings.NewReader(svgContent))
	if err != nil {
		return err
	}

	rectangle := image.Rect(0, 0, logoSizePixmap, logoSizePixmap)
	image := image.NewRGBA(rectangle)
	dasher := rasterx.NewDasher(
		logoSizePixmap,
		logoSizePixmap,
		rasterx.NewScannerGV(
			logoSizePixmap,
			logoSizePixmap,
			image,
			rectangle,
		),
	)
	svgIcon.SetTarget(0, 0, logoSizePixmap, logoSizePixmap)
	svgIcon.Draw(dasher, 1.0)

	k.logo = mo.Some(image)

	return k.updateTray()
}

func (k *Keytray) AddQuit() mo.Result[<-chan struct{}] {
	ch := make(chan struct{}, 1)
	group, err := k.item.Menu().AddChild(
		tray.MenuItemLabel("Quit"),
		tray.MenuItemShortcut([][]string{{"Q"}}),
		tray.MenuItemIconName("application-exit"),
		tray.MenuItemHandler(func(eventID tray.MenuEventID, data any, timestamp uint32) error {
			slog.Info("Quit activated in tray", "EventID", eventID)
			select {
			case ch <- struct{}{}:
			default:
			}
			return nil
		}),
	)
	if err != nil {
		return mo.Err[<-chan struct{}](err)
	}

	k.quitItem = mo.Some(group)

	return mo.Ok((<-chan struct{})(ch))
}

func (k *Keytray) Close() error {
	return k.item.Close()
}
