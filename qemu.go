package main

import (
	"encoding/json"
	"fmt"
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/google/gousb"
	"log"
	"time"
)

type StatusResult struct {
	ID     string `json:"id"`
	Return struct {
		Running    bool   `json:"running"`
		Singlestep bool   `json:"singlestep"`
		Status     string `json:"status"`
	} `json:"return"`
}

type DeviceAddArguments struct {
	Driver    string   `json:"driver"`
	Id        string   `json:"id"`
	VendorId  gousb.ID `json:"-"`
	ProductId gousb.ID `json:"-"`
	BusId     string   `json:"hostbus"`
	PortPath  string   `json:"hostport"`
}

type CommandWithArgs struct {
	Execute   string             `json:"execute"`
	Arguments DeviceAddArguments `json:"arguments"`
}

func status(monitor *qmp.SocketMonitor) (err error) {
	cmd := []byte(`{ "execute": "query-status" }`)
	var raw []byte
	raw, err = monitor.Run(cmd)
	if err != nil {
		return
	}

	var result StatusResult
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return
	}
	//fmt.Println(result.Return.Status)
	return nil
}

func Connect(from string) (monitor *qmp.SocketMonitor, err error) {
	monitor, err = qmp.NewSocketMonitor("unix", "/var/run/qemu-server/"+from+".qmp", 2*time.Second)
	if err != nil {
		log.Printf("Got error connecting to %s: %s", from, err)
		return
	}
	if monitor.Connect() != nil {
		_ = monitor.Disconnect()
		return
	}
	err = status(monitor)
	if err != nil {
		_ = monitor.Disconnect()
		return
	}
	return
}

func ReconnectDevicesToCorrectVM(positions []locationStruct, targetPositionStruct locationStruct, targetDevices []TargetDevice, scannedDevices []ScannedDevice) {
	disconnectDevices(positions, targetPositionStruct, targetDevices)
	connectDevices(targetPositionStruct, targetDevices, scannedDevices)
}

func disconnectDevices(positions []locationStruct, targetPositionStruct locationStruct, targetDevices []TargetDevice) {
	// Disconnect all target devices from other hosts
	for _, position := range positions {
		if position.monitor != nil && targetPositionStruct != position {
			for idx := range targetDevices {
				_, err := position.monitor.Run([]byte(fmt.Sprintf(`{"execute":"device_del", "arguments":{"id":"auto_%d"}}`, idx)))
				if err != nil {
					if err.Error() != fmt.Sprintf("Device 'auto_%d' not found", idx) {
						log.Fatalf("Could not disconnect auto_%d from %s", idx, position.vmId)
					}
				} else {
					log.Printf("Successfully disconnected device auto_%d\n", idx)
				}
			}
		}
	}
}

func connectDevices(targetPositionStruct locationStruct, targetDevices []TargetDevice, scannedDevices []ScannedDevice) {
	// Connect to the target host
	for idx, targetDevice := range targetDevices {
		for _, scannedDevice := range scannedDevices {
			if scannedDevice.vidPid == targetDevice.vidPid {
				bytes, err := json.Marshal(CommandWithArgs{
					Execute: "device_add",
					Arguments: DeviceAddArguments{
						Driver:   "usb-host",
						Id:       fmt.Sprintf("auto_%d", idx),
						BusId:    scannedDevice.busId,
						PortPath: scannedDevice.portPath,
					},
				})
				if err != nil {
					log.Fatalf("Could not serialize %s", err)
				}
				_, err = targetPositionStruct.monitor.Run(bytes)
				if err != nil {
					log.Fatalf("Could not connect device %s (auto_%d) to %s", targetDevice.vidPid, idx, targetPositionStruct.vmId)
				} else {
					log.Printf("Connected %s to auto_%d on VM: %s", targetDevice.vidPid, idx, targetPositionStruct.vmId)
				}
			}
		}
	}
}
