// Copyright (c) 2017-2018 Intel Corporation
// Copyright (c) 2018 Huawei Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package drivers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kata-containers/kata-containers/src/runtime/pkg/device/api"
	"github.com/kata-containers/kata-containers/src/runtime/pkg/device/config"
	"github.com/sirupsen/logrus"
)

const (
	intMax = ^uint(0)

	PCIDomain   = "0000"
	PCIeKeyword = "PCIe"

	PCIConfigSpaceSize = 256
)

type PCISysFsType string

var (
	PCISysFsDevices PCISysFsType = "devices" // /sys/bus/pci/devices
	PCISysFsSlots   PCISysFsType = "slots"   // /sys/bus/pci/slots
)

type PCISysFsProperty string

var (
	PCISysFsDevicesClass     PCISysFsProperty = "class"         // /sys/bus/pci/devices/xxx/class
	PCISysFsSlotsAddress     PCISysFsProperty = "address"       // /sys/bus/pci/slots/xxx/address
	PCISysFsSlotsMaxBusSpeed PCISysFsProperty = "max_bus_speed" // /sys/bus/pci/slots/xxx/max_bus_speed
)

func deviceLogger() *logrus.Entry {
	return api.DeviceLogger()
}

// Identify PCIe device by reading the size of the PCI config space
// Plain PCI device have 256 bytes of config space where PCIe devices have 4K
func isPCIeDevice(bdf string) bool {
	if len(strings.Split(bdf, ":")) == 2 {
		bdf = PCIDomain + ":" + bdf
	}

	configPath := filepath.Join(config.SysBusPciDevicesPath, bdf, "config")
	fi, err := os.Stat(configPath)
	if err != nil {
		deviceLogger().WithField("dev-bdf", bdf).WithError(err).Warning("Couldn't stat() configuration space file")
		return false //Who knows?
	}

	// Plain PCI devices have 256 bytes of configuration space,
	// PCI-Express devices have 4096 bytes
	return fi.Size() > PCIConfigSpaceSize
}

// read from /sys/bus/pci/devices/xxx/property
func getPCIDeviceProperty(bdf string, property PCISysFsProperty) string {
	if len(strings.Split(bdf, ":")) == 2 {
		bdf = PCIDomain + ":" + bdf
	}
	propertyPath := filepath.Join(config.SysBusPciDevicesPath, bdf, string(property))
	rlt, err := readPCIProperty(propertyPath)
	if err != nil {
		deviceLogger().WithError(err).WithField("path", propertyPath).Warn("failed to read pci device property")
		return ""
	}
	return rlt
}

func readPCIProperty(propertyPath string) (string, error) {
	var (
		buf []byte
		err error
	)
	if buf, err = os.ReadFile(propertyPath); err != nil {
		return "", fmt.Errorf("failed to read pci sysfs %v, error:%v", propertyPath, err)
	}
	return strings.Split(string(buf), "\n")[0], nil
}

func GetVFIODeviceType(deviceFilePath string) (config.VFIODeviceType, error) {
	deviceFileName := filepath.Base(deviceFilePath)

	//For example, 0000:04:00.0
	tokens := strings.Split(deviceFileName, ":")
	if len(tokens) == 3 {
		return config.VFIOPCIDeviceNormalType, nil
	}

	//For example, 83b8f4f2-509f-382f-3c1e-e6bfe0fa1001
	tokens = strings.Split(deviceFileName, "-")
	if len(tokens) != 5 {
		return config.VFIODeviceErrorType, fmt.Errorf("Incorrect tokens found while parsing VFIO details: %s", deviceFileName)
	}

	deviceSysfsDev, err := GetSysfsDev(deviceFilePath)
	if err != nil {
		return config.VFIODeviceErrorType, err
	}

	if strings.HasPrefix(deviceSysfsDev, vfioAPSysfsDir) {
		return config.VFIOAPDeviceMediatedType, nil
	}

	return config.VFIOPCIDeviceMediatedType, nil
}

// GetSysfsDev returns the sysfsdev of mediated device
// Expected input string format is absolute path to the sysfs dev node
// eg. /sys/kernel/iommu_groups/0/devices/f79944e4-5a3d-11e8-99ce-479cbab002e4
func GetSysfsDev(sysfsDevStr string) (string, error) {
	return filepath.EvalSymlinks(sysfsDevStr)
}

// GetAPVFIODevices retrieves all APQNs associated with a mediated VFIO-AP
// device
func GetAPVFIODevices(sysfsdev string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(sysfsdev, "matrix"))
	if err != nil {
		return []string{}, err
	}
	// Split by newlines, omitting final newline
	return strings.Split(string(data[:len(data)-1]), "\n"), nil
}
