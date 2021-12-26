package main

import (
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/jessevdk/go-flags"
	"hotplug-proxmox-devices/lib"
	"log"
	"os"
	"regexp"
)

var opts struct {
	DetectDevice  string   `short:"d" long:"detect-device" description:"Device used to detect where which input is active"`
	Positions     []string `short:"p" long:"detect-device-bud" description:"Map VMid to bus of device. In the format of vmid:bus.port[s] e.g. '100:5-2.1.1'. Should be repeated"`
	TargetDevices []string `short:"t" long:"target-device" description:"Device(s) to move. Should be repeated. In the format of '1234:abcd' or '1234:'"`
	ReversedMatch bool     `short:"r" long:"reverse" description:"Reverse match so that all devices not listed will be moved"`
}

type LocationStruct struct {
	Monitor  *qmp.SocketMonitor
	VmId     string
	BusId    string
	PortPath string
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
		tmp, err := lib.Connect(position.VmId)
		if err != nil {
			log.Printf("Could not connect to vmid %s", position.VmId)
		} else {
			//defer func(fromMonitor *qmp.SocketMonitor) {
			//	_ = fromMonitor.Disconnect()
			//}(tmp)
			optsPositions[idx].Monitor = tmp
		}
	}

	detectAndReconnect(optsDetectDevice, optsPositions, optsTargetDevices, opts.ReversedMatch)
	//runOnDeviceChanged(func() {
	//	detectAndReconnect(optsDetectDevice, optsPositions, optsTargetDevices)
	//})
}

func detectAndReconnect(optsDetectDevice string, optsPositions []LocationStruct, optsTargetDevices []lib.TargetDevice, optsReversedMatch bool) {
	// Get list of all interesting devices
	foundDevices := lib.ScanUsbDevices()

	// Find where the device should be connected
	var targetPositionStruct LocationStruct
	for _, device := range foundDevices {
		if device.VidPid == optsDetectDevice {
			for _, position := range optsPositions {
				if position.BusId == device.BusId && position.PortPath == device.PortPath {
					targetPositionStruct = position
					break
				}
			}
			if targetPositionStruct.Monitor == nil {
				log.Fatalf("Failed to find device. It was on %s-%s, but that's not among the options", device.BusId, device.PortPath)
			}
		}
	}
	if targetPositionStruct.Monitor == nil {
		log.Fatalf("There is no connection for target VM. It may not even have been found.")
	}

	namedConnections := make([]lib.NamedConnection, len(opts.TargetDevices))
	for idx, device := range optsPositions {
		namedConnections[idx] = lib.NamedConnection{
			VmId:    device.VmId,
			Monitor: device.Monitor,
		}
	}

	lib.ReconnectDevicesToCorrectVM(
		namedConnections,
		lib.NamedConnection{
			VmId:    targetPositionStruct.VmId,
			Monitor: targetPositionStruct.Monitor,
		},
		optsTargetDevices,
		foundDevices,
		optsReversedMatch,
	)
}

func parseOptsTargetDevices() []lib.TargetDevice {
	targetDevices := make([]lib.TargetDevice, len(opts.TargetDevices))
	regex := regexp.MustCompile("^([0-9a-f]{4}):([0-9a-f]{4})?$")
	for idx, device := range opts.TargetDevices {

		if !regex.MatchString(device) {
			log.Fatalf("%s is not valid format", device)
		}
		targetDevices[idx] = lib.TargetDevice{
			VidPid: device,
		}
	}
	return targetDevices
}

func locationOptsToStructs() (positions []LocationStruct) {
	regex := regexp.MustCompile("^([0-9]+):([0-9]+)-([0-9.]+)$")
	for _, position := range opts.Positions {
		match := regex.FindStringSubmatch(position)
		if len(match) == 4 {
			positions = append(
				positions,
				LocationStruct{
					Monitor:  nil,
					VmId:     match[1],
					BusId:    match[2],
					PortPath: match[3],
				},
			)
		}
	}
	return positions
}
