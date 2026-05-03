package usb

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/samber/mo"
)

var (
	usbInterfaceRegexp    = regexp.MustCompile(`^\d+-\d+(\.\d+)*:\d+\.\d+$`)
	ErrHardwareIDNotFound = errors.New("hardware ID not found")
)

func GetHardwareID(path string) mo.Result[string] {
	deviceName := filepath.Base(path)
	sysPath, err := filepath.EvalSymlinks(filepath.Join("/sys/class/hidraw", deviceName, "device"))
	if err != nil {
		return mo.Err[string](err)
	}
	segments := strings.Split(sysPath, "/")

	for i := len(segments) - 1; i >= 0; i-- {
		if usbInterfaceRegexp.MatchString(segments[i]) {
			return mo.Ok(strings.Split(segments[i], ":")[0])
		}
	}
	return mo.Err[string](ErrHardwareIDNotFound)
}
