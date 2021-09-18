package main

import (
	"encoding/json"
	"fmt"
	"github.com/digitalocean/go-qemu/qmp"
	"log"
	"strings"
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

type CommandWithArgs struct {
	Execute   string            `json:"execute"`
	Arguments map[string]string `json:"arguments"`
}

type ConnectedDevice struct {
	// e.g. auto_0
	connectedName string
	qomTreePath   string
	otherPath     string
	busAndPort    string
}

type QomListProperties struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type QomListResponse struct {
	Return []QomListProperties `json:"return"`
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

func ReconnectDevicesToCorrectVM(
	positions []locationStruct,
	targetPositionStruct locationStruct,
	targetDevices []TargetDevice,
	scannedDevices []ScannedDevice,
	reverseMatch bool,
) {
	mappedDevices := make(map[string]ConnectedDevice)
	for _, device := range ListConnectedDevices(targetPositionStruct.monitor) {
		if device.connectedName != "" {
			mappedDevices[device.connectedName] = device
		}
	}
	time.Sleep(time.Duration(60e9))
	disconnectDevices(positions, targetDevices, mappedDevices, targetPositionStruct)
	// Give qemu a couple of seconds to settle before connecting
	time.Sleep(time.Duration(3e9))
	connectDevices(targetPositionStruct, targetDevices, scannedDevices, mappedDevices)
}

func disconnectDevices(positions []locationStruct, targetDevices []TargetDevice, devices map[string]ConnectedDevice, targetPositionStruct locationStruct) {
	// Disconnect all target devices from other hosts
	for _, position := range positions {
		if position.monitor != nil {
			for idx, targetDevice := range targetDevices {
				deviceName := fmt.Sprintf("auto_%d", idx)
				if position == targetPositionStruct && devices[deviceName].connectedName != "" {
					continue
				}
				log.Printf("Disconnecting device %s from vm %s %s", targetDevice.vidPid, position.vmId, deviceName)
				disconnectDevice(position, deviceName)
			}
		}
	}
}

func disconnectDevice(position locationStruct, deviceName string) {
	_, err := position.monitor.Run([]byte(fmt.Sprintf(`{"execute":"device_del", "arguments":{"id":"%s"}}`, deviceName)))
	if err != nil {
		if err.Error() != fmt.Sprintf("Device '%s' not found", deviceName) {
			log.Fatalf("Could not disconnect %s from %s", deviceName, position.vmId)
		}
	} else {
		log.Printf("Successfully disconnected device %s from %s\n", deviceName, position.vmId)
	}
}

func QmpRun(monitor *qmp.SocketMonitor, args CommandWithArgs, v interface{}) (err error) {
	bytes, err := json.Marshal(args)
	if err != nil {
		log.Fatalf("Could not serialize %s", err)
	}
	resultBytes, err := monitor.Run(bytes)
	if err != nil {
		return
	}
	err = json.Unmarshal(resultBytes, v)
	if err != nil {
		log.Fatalf("Could not deserialize %s because of %s", string(resultBytes), err)
	}
	return
}

func connectDevices(targetPositionStruct locationStruct, targetDevices []TargetDevice, scannedDevices []ScannedDevice, devices map[string]ConnectedDevice) {
	// Connect to the target host
	for idx, targetDevice := range targetDevices {
		for _, scannedDevice := range scannedDevices {
			if strings.HasPrefix(scannedDevice.vidPid, targetDevice.vidPid) {
				deviceName := fmt.Sprintf("auto_%d", idx)
				if devices[deviceName].connectedName != "" {
					continue
				}
				log.Printf("Connecting device %s to %s", scannedDevice.vidPid, deviceName)
				time.Sleep(time.Duration(5e9))
				connectDevice(targetPositionStruct, deviceName, scannedDevice.busId, scannedDevice.portPath, targetDevice, idx)
			}
		}
	}
}

func connectDevice(targetPositionStruct locationStruct, deviceName string, hostBus string, hostPort string, targetDevice TargetDevice, idx int) {
	bytes, err := json.Marshal(CommandWithArgs{
		Execute: "device_add",
		Arguments: map[string]string{
			"driver":   "usb-host",
			"id":       deviceName,
			"hostbus":  hostBus,
			"hostport": hostPort,
		},
	})
	if err != nil {
		log.Fatalf("Could not serialize %s", err)
	}
	_, err = targetPositionStruct.monitor.Run(bytes)
	if err != nil {
		log.Printf("Could not connect device %s (auto_%d) to %s, %s", targetDevice.vidPid, idx, targetPositionStruct.vmId, err)
	} else {
		log.Printf("Connected %s to auto_%d on VM: %s", targetDevice.vidPid, idx, targetPositionStruct.vmId)
	}
}

func QomList(monitor *qmp.SocketMonitor, path string) (properties []QomListProperties) {
	var response QomListResponse
	err := QmpRun(
		monitor,
		CommandWithArgs{
			Execute: "qom-list",
			Arguments: map[string]string{
				"path": path,
			},
		},
		&response,
	)
	if err != nil {
		log.Fatalf("Could not get qom-list %s due to %s", path, err)
	}
	return response.Return
}

func QomGetInt(monitor *qmp.SocketMonitor, path string, property string) (value int64) {
	response := make(map[string]int64)
	err := QmpRun(
		monitor,
		CommandWithArgs{
			Execute: "qom-get",
			Arguments: map[string]string{
				"path":     path,
				"property": property,
			},
		},
		&response,
	)
	if err != nil {
		log.Fatalf("Could not get property %s %s due to %s", path, property, err)
	}
	return response["return"]
}

func QomGetString(monitor *qmp.SocketMonitor, path string, property string) (value string) {
	response := make(map[string]string)
	err := QmpRun(
		monitor,
		CommandWithArgs{
			Execute: "qom-get",
			Arguments: map[string]string{
				"path":     path,
				"property": property,
			},
		},
		&response,
	)
	if err != nil {
		log.Fatalf("Could not get property %s %s due to %s", path, property, err)
	}
	return response["return"]
}

func ListConnectedDevices(
	monitor *qmp.SocketMonitor,
) (connectedDevices []ConnectedDevice) {
	path := "/machine/q35/pcie.0"
	for _, usbRootDevice := range QomList(monitor, path) {
		if !strings.Contains(usbRootDevice.Type, "usb") {
			continue
		}
		rootUsbPath := path + "/" + usbRootDevice.Name
		for _, usbBus := range QomList(monitor, rootUsbPath) {
			if usbBus.Type != "child<usb-bus>" {
				continue
			}
			busPath := rootUsbPath + "/" + usbBus.Name
			for _, connectedUsbDevice := range QomList(monitor, busPath) {
				if connectedUsbDevice.Type != "link<usb-host>" {
					continue
				}
				devicePath := busPath + "/" + connectedUsbDevice.Name
				otherPath := QomGetString(monitor, busPath, connectedUsbDevice.Name)

				hostBus := QomGetInt(monitor, devicePath, "hostbus")
				hostPort := QomGetString(monitor, devicePath, "hostport")

				connectedName := otherPath[strings.LastIndex(otherPath, "/")+1:]
				busAndPort := fmt.Sprintf("%d-%s", hostBus, hostPort)
				log.Printf("Found device %s @%s with name %s", devicePath, busAndPort, connectedName)

				connectedDevices = append(
					connectedDevices,
					ConnectedDevice{
						connectedName: connectedName,
						qomTreePath:   devicePath,
						otherPath:     otherPath,
						busAndPort:    busAndPort,
					},
				)
			}
		}
	}
	return
}
