package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"hotplug-proxmox-devices/lib"
	"log"
	"os"
)

type ConnectedDeviceWithVidPid struct {
	// e.g. auto_0
	ConnectedName string `yaml:"connectedName"`
	QomTreePath   string `yaml:"qomTreePath"`
	OtherPath     string `yaml:"otherPath"`
	BusAndPort    string `yaml:"busAndPort"`
	VidPid        string `yaml:"vidPid"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("%s <vmId>", os.Args[0])
	}
	vmId := os.Args[1]

	tmp, err := lib.Connect(vmId)
	if err != nil {
		log.Fatalf("Could not connect to vmid %s", vmId)
	}
	connection := lib.NamedConnection{
		Monitor: tmp,
		VmId:    vmId,
	}
	scannedDevices := lib.ScanUsbDevices()
	connectedDevices := lib.ListConnectedDevices(connection)
	devices := make([]ConnectedDeviceWithVidPid, len(connectedDevices))
	for idx, device := range devices {
		var vidPid string
		for _, scannedDevice := range scannedDevices {
			busAndPort := fmt.Sprintf("%s-%s", scannedDevice.BusId, scannedDevice.PortPath)
			if busAndPort == device.BusAndPort {
				vidPid = device.VidPid
				break
			} else {
				log.Printf("%s vs %s", busAndPort, device.BusAndPort)
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

	err = yaml.NewEncoder(log.Writer()).Encode(connectedDevices)
	if err != nil {
		log.Fatalf("%s", err)
	}

}
