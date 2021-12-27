package main

import (
	"gopkg.in/yaml.v3"
	"hotplug-proxmox-devices/lib"
	"log"
	"os"
)

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
	connectedDevices := lib.ListConnectedDevices(connection)
	yaml.NewEncoder(log.Writer()).Encode(connectedDevices)
}
