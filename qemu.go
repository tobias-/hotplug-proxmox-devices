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
	VendorId  gousb.ID `json:"vendorid"`
	ProductId gousb.ID `json:"productid"`
	BusId     int64    `json:"-"`
	Port 	  string   `json:"port"`
	Address   gousb.ID `json:"hostaddr"`
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

func ReconnectDevicesToCorrectVM(positions []positionStruct, targetPositionStruct positionStruct, targetDevices []TargetDevice) {
	disconnectDevices(positions, targetPositionStruct, targetDevices)
	connectDevices(targetPositionStruct, targetDevices)
}

func disconnectDevices(positions []positionStruct, targetPositionStruct positionStruct, targetDevices []TargetDevice) {
	// Disconnect all target devices from other hosts
	for _, position := range positions {
		if position.monitor != nil && targetPositionStruct != position {
			for idx, _ := range targetDevices {
				_, err := position.monitor.Run([]byte(fmt.Sprintf(`{"execute":"device_del", "arguments":{"id":"auto_%d"}}`, idx)))
				if err != nil {
					log.Fatalf("Could not disconnect auto_%d from %s", idx, position.vmId)
				}
			}
		}
	}
}

func connectDevices(targetPositionStruct positionStruct, targetDevices []TargetDevice) {
	// Connect to the target host
	for idx, targetDevice := range targetDevices {
		bytes, err := json.Marshal(CommandWithArgs{
			Execute: "device_add",
			Arguments: DeviceAddArguments{
				Driver:    "usb-host",
				Id:        fmt.Sprintf("auto_%d", idx),
				VendorId:  targetDevice.vendorId,
				ProductId: targetDevice.productId,
			},
		})
		if err != nil {
			log.Fatalf("Could not connect auto_%d to %s", idx, targetPositionStruct.vmId)
		}
		_, err = targetPositionStruct.monitor.Run(bytes)
		if err != nil {
			log.Fatalf("Could not connect device %s (auto_%d) to %s", targetDevice.vidPid, idx, targetPositionStruct.vmId)
		}
	}
}
