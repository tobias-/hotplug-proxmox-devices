package main

import (
	"github.com/jessevdk/go-flags"
	"github.com/tobias-/hotplug-proxmox-devices/lib"
	"log"
	"os"
)

var opts struct {
	Devices []string `short:"d" long:"device" description:"Device to connect (repeatable). use list_connected_devices to and use the 'connectedName'"`
	VmId    string   `short:"m" long:"vmid" description:"VmId to connect device to"`
}

func main() {
	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		log.Fatal(err)
	}

	tmp, err := lib.Connect(opts.VmId)
	if err != nil {
		log.Fatalf("Could not connect to vmid %s", opts.VmId)
	}
	connection := lib.NamedConnection{
		Monitor: tmp,
		VmId:    opts.VmId,
	}

	for _, device := range opts.Devices {
		lib.DisconnectDevice(connection, device)
	}
}
