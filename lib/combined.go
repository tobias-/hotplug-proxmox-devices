package lib

import (
	"fmt"
)

type ConnectedDeviceWithVidPid struct {
	// e.g. auto_0
	ConnectedName string `yaml:"connectedName"`
	QomTreePath   string `yaml:"qomTreePath"`
	OtherPath     string `yaml:"otherPath"`
	BusAndPort    string `yaml:"busAndPort"`
	VidPid        string `yaml:"vidPid"`
}

func ListConnectedDevicesWithVidPid(connection NamedConnection) (ConnectedDevices []ConnectedDeviceWithVidPid) {
	scannedDevices := ScanUsbDevices()
	connectedDevices := ListConnectedDevices(connection)
	devices := make([]ConnectedDeviceWithVidPid, len(connectedDevices))
	for idx, device := range connectedDevices {
		var vidPid string
		for _, scannedDevice := range scannedDevices {
			busAndPort := fmt.Sprintf("%s-%s", scannedDevice.BusId, scannedDevice.PortPath)
			if busAndPort == device.BusAndPort {
				vidPid = scannedDevice.VidPid
				break
			}
		}

		devices[idx] = ConnectedDeviceWithVidPid{
			ConnectedName: connectedDevices[idx].ConnectedName,
			QomTreePath:   connectedDevices[idx].ConnectedName,
			OtherPath:     connectedDevices[idx].ConnectedName,
			BusAndPort:    connectedDevices[idx].ConnectedName,
			VidPid:        vidPid,
		}
	}
	return devices
}
