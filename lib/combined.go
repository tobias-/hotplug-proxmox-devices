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
	Product       string `yaml:"product"`
}

func ListConnectedDevicesWithVidPid(connection NamedConnection) (ConnectedDevices []ConnectedDeviceWithVidPid) {
	scannedDevices := ScanUsbDevices()
	connectedDevices := ListConnectedDevices(connection)
	devices := make([]ConnectedDeviceWithVidPid, len(connectedDevices))
	for idx, device := range connectedDevices {
		var foundScannedDevice ScannedDevice
		for _, scannedDevice := range scannedDevices {
			busAndPort := fmt.Sprintf("%s-%s", scannedDevice.BusId, scannedDevice.PortPath)
			if busAndPort == device.BusAndPort {
				foundScannedDevice = scannedDevice
				break
			}
		}

		devices[idx] = ConnectedDeviceWithVidPid{
			ConnectedName: connectedDevices[idx].ConnectedName,
			QomTreePath:   connectedDevices[idx].QomTreePath,
			OtherPath:     connectedDevices[idx].OtherPath,
			BusAndPort:    connectedDevices[idx].BusAndPort,
			VidPid:        foundScannedDevice.VidPid,
			Product:       foundScannedDevice.Product,
		}
	}
	return devices
}
