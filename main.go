package main

import (
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/jessevdk/go-flags"
	"log"
	"os"
	"regexp"
)

type TargetDevice struct {
	vidPid string
}

var opts struct {
	DetectDevice  string   `short:"d" long:"detect-device" description:"Device used to detect where which input is active"`
	Positions     []string `short:"p" long:"detect-device-bud" description:"Map VMid to bus of device. In the format of vmid:bus.port[s] e.g. '100:5-2.1.1'. Should be repeated"`
	TargetDevices []string `short:"t" long:"target-device" description:"Device(s) to move. Should be repeated. In the format of '1234:abcd'"`
}

type locationStruct struct {
	monitor  *qmp.SocketMonitor
	vmId     string
	busId    string
	portPath string
}

func main() {
	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		log.Fatal(err)
	}
	optsPositions := locationOptsToStructs()
	optsTargetDevices := parseOptsTargetDevices()
	optsDetectDevice := opts.DetectDevice

	// Connect all
	for idx, position := range optsPositions {
		tmp, err := Connect(position.vmId)
		if err != nil {
			log.Printf("Could not connect to vmid %s", position.vmId)
		} else {
			//defer func(fromMonitor *qmp.SocketMonitor) {
			//	_ = fromMonitor.Disconnect()
			//}(tmp)
			optsPositions[idx].monitor = tmp
		}
	}

	detectAndReconnect(optsDetectDevice, optsPositions, optsTargetDevices)
	//runOnDeviceChanged(func() {
	//	detectAndReconnect(optsDetectDevice, optsPositions, optsTargetDevices)
	//})
}

func detectAndReconnect(optsDetectDevice string, optsPositions []locationStruct, optsTargetDevices []TargetDevice) {
	// Get list of all interesting devices
	foundDevices := scanUsbDevices()

	// Find where the device should be connected
	var targetPositionStruct locationStruct
	for _, device := range foundDevices {
		if device.vidPid == optsDetectDevice {
			for _, position := range optsPositions {
				if position.busId == device.busId && position.portPath == device.portPath {
					targetPositionStruct = position
					break
				}
			}
			if targetPositionStruct.monitor == nil {
				log.Fatalf("Failed to find device. It was on %s-%s, but that's not among the options", device.busId, device.portPath)
			}
		}
	}
	if targetPositionStruct.monitor == nil {
		log.Fatalf("There is no connection for target VM. It may not even have been found.")
	}

	ReconnectDevicesToCorrectVM(optsPositions, targetPositionStruct, optsTargetDevices, foundDevices)
}

func parseOptsTargetDevices() []TargetDevice {
	targetDevices := make([]TargetDevice, len(opts.TargetDevices))
	regex := regexp.MustCompile("^([0-9a-f]{4}):([0-9a-f]{4})$")
	for idx, device := range opts.TargetDevices {

		if !regex.MatchString(device) {
			log.Fatalf("%s is not valid format", device)
		}
		targetDevices[idx] = TargetDevice{
			vidPid: device,
		}
	}
	return targetDevices
}

func locationOptsToStructs() (positions []locationStruct) {
	regex := regexp.MustCompile("^([0-9]+):([0-9]+)-([0-9.]+)$")
	for _, position := range opts.Positions {
		match := regex.FindStringSubmatch(position)
		if len(match) == 4 {
			positions = append(
				positions,
				locationStruct{
					monitor:  nil,
					vmId:     match[1],
					busId:    match[2],
					portPath: match[3],
				},
			)
		}
	}
	return positions
}
