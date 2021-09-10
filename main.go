package main

// #cgo CFLAGS: -g -Werr
// #include <libusb/libusb.h>

import "C"
import (
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/google/gousb"
	"github.com/jessevdk/go-flags"
	"log"
	"os"
	"strconv"
	"strings"
)

type TargetDevice struct {
	vendorId  gousb.ID
	productId gousb.ID
	vidPid    string
}

var opts struct {
	DetectDevice  string   `short:"d" long:"detect-device" description:"Device used to detect where which input is active"`
	Positions     []string `short:"p" long:"detect-device-bud" description:"Map VMid to bus of device. In the format of vmid:bus.port[s] e.g. '100:5-2.1.1'. Should be repeated"`
	TargetDevices []string `short:"t" long:"target-device" description:"Device(s) to move. Should be repeated"`
}

type positionStruct struct {
	monitor *qmp.SocketMonitor
	vmId    string
	busId   int64
}

func main() {
	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		log.Fatal(err)
	}
	positions := convertToStructs()
	targetDevices := parseTargetDevices()

	// Get list of all interesting devices
	foundDevices, err := scanDevices()
	if err != nil {
		log.Fatalf("Could not scan devices: %s", err)
	}

	// Connect all
	for _, position := range positions {
		tmp, err := Connect(position.vmId)
		if err != nil {
			log.Printf("Could not connect to vmid %s", position.vmId)
		} else {
			defer func(fromMonitor *qmp.SocketMonitor) {
				_ = fromMonitor.Disconnect()
			}(tmp)
			position.monitor = tmp
		}
	}

	// Find where the device should be connected
	var targetPositionStruct positionStruct
	for _, device := range foundDevices {
		if toVidPid(device) == opts.DetectDevice {
			for _, position := range positions {
				if position.busId == device.BusId {
					targetPositionStruct = position
					break
				}
			}
			if targetPositionStruct.monitor == nil {
				log.Fatalf("There is no connection for target VM (so we cannot send the device there). We found the detect device(%s) on bus %d", toVidPid(device), device.BusId)
			}
		}
	}
	if targetPositionStruct.monitor == nil {
		log.Fatalf("There is no connection for target VM. It may not even have been found.")
	}

	ReconnectDevicesToCorrectVM(positions, targetPositionStruct, targetDevices)
}

func parseTargetDevices() []TargetDevice {
	targetDevices := make([]TargetDevice, len(opts.TargetDevices))
	for idx, device := range opts.TargetDevices {
		split := strings.SplitN(device, ":", 3)
		if len(split) != 2 {
			log.Fatalf("%s is not valid format", device)
		}
		vid, err := strconv.ParseInt(split[0], 16, 32)
		if err != nil {
			log.Fatalf("Could not parse %s. Expected format 1a2b:3c4d", device)
		}
		pid, err := strconv.ParseInt(split[1], 16, 32)
		if err != nil {
			log.Fatalf("Could not parse %s. Expected format 1a2b:3c4d", device)
		}
		targetDevices[idx] = TargetDevice{
			vendorId:  gousb.ID(vid),
			productId: gousb.ID(pid),
			vidPid:    device,
		}
	}
	return targetDevices
}

func convertToStructs() []positionStruct {
	positions := make([]positionStruct, len(opts.Positions))
	for idx, position := range opts.Positions {
		split := strings.SplitN(position, ":", 3)
		if len(split) != 2 {
			log.Fatalf("%s is not valid format", position)
		}
		value, err := strconv.ParseInt(split[1], 10, 32)
		if err != nil {
			log.Fatalf("Couldn't parse second half of %s as decimal int", position)
		}
		positions[idx] = positionStruct{
			nil,
			split[0],
			value,
		}
	}
	return positions
}

func indexOf(s []string, str string) (idx int) {
	for idx, v := range s {
		if v == str {
			return idx
		}
	}
	return -1
}
