package main

// #cgo CFLAGS: -g -Werr
// #include <libusb/libusb.h>

import "C"
import (
	"encoding/json"
	"fmt"
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/google/gousb"
	"github.com/google/gousb/usbid"
	"github.com/jessevdk/go-flags"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type TargetDevice struct {
	vendorId  gousb.ID
	productId gousb.ID
	vidPid    string
}

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
}

type CommandWithArgs struct {
	Execute   string             `json:"execute"`
	Arguments DeviceAddArguments `json:"arguments"`
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
		tmp, err := connect(position.vmId)
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

func toVidPid2(vendorId gousb.ID, productId gousb.ID) string {
	return fmt.Sprintf("%04d:%04d", vendorId, productId)
}

func toVidPid(dev DeviceAddArguments) string {
	return fmt.Sprintf("%04d:%04d", dev.VendorId, dev.ProductId)
}

func scanDevices() (result []DeviceAddArguments, err error) {
	ctx := gousb.NewContext()
	defer func(ctx *gousb.Context) {
		_ = ctx.Close()
	}(ctx)

	devs, err := ctx.OpenDevices(func(descriptor *gousb.DeviceDesc) bool {
		fmt.Printf("%03d.%03d %s:%s %s Port: %d\n", descriptor.Bus, descriptor.Address, descriptor.Vendor, descriptor.Product, usbid.Describe(descriptor), descriptor.Port)
		fmt.Printf("  Protocol: %s\n", usbid.Classify(descriptor))
		vidPid := toVidPid2(descriptor.Vendor, descriptor.Product)
		return indexOf(opts.TargetDevices, vidPid) > 0 || vidPid == opts.DetectDevice
	})
	if err != nil {
		log.Fatalf("list devices: %s", err)
	}
	defer func() {
		for _, d := range devs {
			_ = d.Close()
		}
	}()

	result = make([]DeviceAddArguments, len(devs))
	for i, dev := range devs {
		descriptor := dev.Desc
		//var port [16]byte;
		//C.usb_host_get_port(devs[i], port, 16);
		result[i] = DeviceAddArguments{
			Driver: "usb-host",
			//HostBus:   fmt.Sprintf("%02x", descriptor.Bus),
			VendorId:  descriptor.Vendor,
			ProductId: descriptor.Product,
		}
	}

	for _, d := range devs {
		log.Printf("Found %04x:%04x", d.Desc.Vendor, d.Desc.Product)
	}
	sort.Slice(result, func(i, j int) bool {
		iVid := result[i].VendorId
		jVid := result[j].VendorId
		if iVid < jVid {
			return true
		} else if iVid > jVid {
			return false
		}
		iPid := result[i].ProductId
		jPid := result[j].ProductId
		if iPid < jPid {
			return true
		} else if iPid > jPid {
			return false
		}
		return false
	})
	return
}

func connect(from string) (monitor *qmp.SocketMonitor, err error) {
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

func indexOf(s []string, str string) (idx int) {
	for idx, v := range s {
		if v == str {
			return idx
		}
	}
	return -1
}
