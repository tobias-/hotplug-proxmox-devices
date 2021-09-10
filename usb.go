package main

import (
	"fmt"
	"github.com/google/gousb"
	"github.com/google/gousb/usbid"
	"log"
	"sort"
)

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


