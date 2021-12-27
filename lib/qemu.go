package lib

import (
	"encoding/json"
	"fmt"
	"github.com/digitalocean/go-qemu/qmp"
	"log"
	"math/rand"
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
	ConnectedName string `yaml:"connectedName"`
	QomTreePath   string `yaml:"qomTreePath"`
	OtherPath     string `yaml:"otherPath"`
	BusAndPort    string `yaml:"busAndPort"`
}

type QomListProperties struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type QomListResponse struct {
	Return []QomListProperties `json:"return"`
}

type TargetDevice struct {
	VidPid string
}

type NamedConnection struct {
	Monitor *qmp.SocketMonitor
	VmId    string
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

// Connect to a vm's qemu socket
func Connect(vmId string) (monitor *qmp.SocketMonitor, err error) {
	monitor, err = qmp.NewSocketMonitor("unix", "/var/run/qemu-server/"+vmId+".qmp", 2*time.Second)
	if err != nil {
		log.Printf("Got error connecting to %s: %s", vmId, err)
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
	positions []NamedConnection,
	targetPositionStruct NamedConnection,
	targetDevices []TargetDevice,
	scannedDevices []ScannedDevice,
	reverseMatch bool,
) {
	mappedDevices := make(map[string]ConnectedDevice)
	for _, device := range ListConnectedDevices(targetPositionStruct) {
		if device.ConnectedName != "" {
			mappedDevices[device.ConnectedName] = device
		}
	}
	disconnectDevices(positions, targetDevices, mappedDevices, targetPositionStruct)
	ConnectDevices(targetPositionStruct, targetDevices, scannedDevices, mappedDevices)
}

func disconnectDevices(clearedConnections []NamedConnection, targetDevices []TargetDevice, devices map[string]ConnectedDevice, targetPositionStruct NamedConnection) {
	// Disconnect all target devices from other hosts
	for _, position := range clearedConnections {
		if position.Monitor != nil && targetPositionStruct != position {
			for _, device := range ListConnectedDevices(position) {
				log.Printf("Disconnecting device %s from vm %s (host connection: %s)", device.ConnectedName, position.VmId, device.BusAndPort)
				DisconnectDevice(position, device.ConnectedName)
				time.Sleep(time.Duration(1e9))
			}
		}
	}
}

func DisconnectDevice(position NamedConnection, deviceName string) {
	_, err := position.Monitor.Run([]byte(fmt.Sprintf(`{"execute":"device_del", "arguments":{"id":"%s"}}`, deviceName)))
	if err != nil {
		if err.Error() != fmt.Sprintf("Device '%s' not found", deviceName) {
			log.Fatalf("Could not disconnect %s from %s", deviceName, position.VmId)
		}
	} else {
		log.Printf("Successfully disconnected device %s from %s\n", deviceName, position.VmId)
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

func ConnectDevices(targetPositionStruct NamedConnection, targetDevices []TargetDevice, scannedDevices []ScannedDevice, devices map[string]ConnectedDevice) {
	// Connect to the target host
	for idx, targetDevice := range targetDevices {
		for _, scannedDevice := range scannedDevices {
			if strings.HasPrefix(scannedDevice.VidPid, targetDevice.VidPid) {
				deviceName := fmt.Sprintf("auto_%d", idx)
				if devices[deviceName].ConnectedName != "" {
					log.Printf("%s is already connected to %s as %s", scannedDevice.VidPid, targetPositionStruct.VmId, deviceName)
					continue
				}
				log.Printf("Connecting device %s to %s", scannedDevice.VidPid, deviceName)
				time.Sleep(time.Duration(5e9))
				connectDevice(targetPositionStruct, deviceName, scannedDevice.BusId, scannedDevice.PortPath, targetDevice, idx)
			}
		}
	}
}

func ConnectDevice(connection NamedConnection, targetDevice TargetDevice, scannedDevices []ScannedDevice, id int) {
	for _, scannedDevice := range scannedDevices {
		if strings.HasPrefix(scannedDevice.VidPid, targetDevice.VidPid) {
			deviceName := fmt.Sprintf("auto_%d", rand.Int31n(8192))
			log.Printf("Connecting device %s to %s", scannedDevice.VidPid, deviceName)
			time.Sleep(time.Duration(5e9))
			connectDevice(connection, deviceName, scannedDevice.BusId, scannedDevice.PortPath, targetDevice, id)
		}
	}
}

func connectDevice(connection NamedConnection, deviceName string, hostBus string, hostPort string, targetDevice TargetDevice, idx int) {
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
	_, err = connection.Monitor.Run(bytes)
	if err != nil {
		log.Printf("Could not connect device %s (auto_%d) to %s, %s", targetDevice.VidPid, idx, connection.VmId, err)
	} else {
		log.Printf("Connected %s to auto_%d on VM: %s", targetDevice.VidPid, idx, connection.VmId)
	}
}

func QomList(connection NamedConnection, path string) (properties []QomListProperties) {
	var response QomListResponse
	err := QmpRun(
		connection.Monitor,
		CommandWithArgs{
			Execute: "qom-list",
			Arguments: map[string]string{
				"path": path,
			},
		},
		&response,
	)
	if err != nil {
		log.Fatalf("Could not get qom-list %s from %s due to %s", path, connection.VmId, err)
	}
	return response.Return
}

func QomGetInt(connection NamedConnection, path string, property string) (value int64) {
	response := make(map[string]int64)
	err := QmpRun(
		connection.Monitor,
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
		log.Fatalf("Could not get property %s %s from %s due to %s", path, property, connection.VmId, err)
	}
	return response["return"]
}

func QomGetString(connection NamedConnection, path string, property string) (value string) {
	response := make(map[string]string)
	err := QmpRun(
		connection.Monitor,
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
		log.Fatalf("Could not get property %s %s from %s due to %s", path, property, connection.VmId, err)
	}
	return response["return"]
}

func ListConnectedDevices(connection NamedConnection) (connectedDevices []ConnectedDevice) {
	path := "/machine/q35/pcie.0"
	for _, usbRootDevice := range QomList(connection, path) {
		if !strings.Contains(usbRootDevice.Type, "usb") {
			continue
		}
		rootUsbPath := path + "/" + usbRootDevice.Name
		for _, usbBus := range QomList(connection, rootUsbPath) {
			if usbBus.Type != "child<usb-bus>" {
				continue
			}
			busPath := rootUsbPath + "/" + usbBus.Name
			for _, connectedUsbDevice := range QomList(connection, busPath) {
				if connectedUsbDevice.Type != "link<usb-host>" {
					continue
				}
				devicePath := busPath + "/" + connectedUsbDevice.Name
				otherPath := QomGetString(connection, busPath, connectedUsbDevice.Name)

				hostBus := QomGetInt(connection, devicePath, "hostbus")
				hostPort := QomGetString(connection, devicePath, "hostport")

				connectedName := otherPath[strings.LastIndex(otherPath, "/")+1:]
				busAndPort := fmt.Sprintf("%d-%s", hostBus, hostPort)
				//log.Printf("Found device %s on %s @%s with name %s", devicePath, connection.VmId, busAndPort, connectedName)

				connectedDevices = append(
					connectedDevices,
					ConnectedDevice{
						ConnectedName: connectedName,
						QomTreePath:   devicePath,
						OtherPath:     otherPath,
						BusAndPort:    busAndPort,
					},
				)
			}
		}
	}
	return
}
