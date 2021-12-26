package main

import (
	"github.com/jessevdk/go-flags"
	"hotplugDevices/lib"
	"log"
	"math/rand"
	"os"
	"regexp"
)

var opts struct {
	Device []string `short:"d" long:"device" description:"Device(s) to connect"`
	VmId   string   `short:"m" long:"vmid" description:"VmId to connect device to"`
}

func main() {
	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		log.Fatal(err)
	}
	optsDevices := optsDevices()

	tmp, err := lib.Connect(opts.VmId)
	if err != nil {
		log.Fatalf("Could not connect to vmid %s", opts.VmId)
	}
	connection := lib.NamedConnection{
		Monitor: tmp,
		VmId:    opts.VmId,
	}
	foundDevices := lib.ScanUsbDevices()

	for _, device := range optsDevices {
		lib.ConnectDevice(connection, device, foundDevices, int(rand.Int31n(8192)))
	}
}

func optsDevices() []lib.TargetDevice {
	targetDevices := make([]lib.TargetDevice, len(opts.Device))
	regex := regexp.MustCompile("^([0-9a-f]{4}):([0-9a-f]{4})?$")
	for idx, device := range opts.Device {

		if !regex.MatchString(device) {
			log.Fatalf("%s is not valid format", device)
		}
		targetDevices[idx] = lib.TargetDevice{
			VidPid: device,
		}
	}
	return targetDevices
}
