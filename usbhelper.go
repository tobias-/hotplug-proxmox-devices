package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
)

type ScannedDevice struct {
	vidPid    string
	busId     string
	addressId string
	portPath  string
}

func trim(str []byte) (trimmed string) {
	return strings.Trim(string(str), "\n")
}

func scanUsbDevices() (scannedDevices []ScannedDevice) {
	dirname := "/sys/bus/usb/devices"
	dir, err := ioutil.ReadDir(dirname)
	if err != nil {
		log.Fatalf("Error because: %s", err)
	}
	regex := regexp.MustCompile("^([0-9]+)-([0-9.]+)$")
	for _, info := range dir {
		filename := info.Name()
		find := regex.FindStringSubmatch(filename)
		if len(find) == 3 {

			vidPath := fmt.Sprintf("%s/%s/idVendor", dirname, filename)
			vid, err := ioutil.ReadFile(vidPath)
			if err != nil {
				log.Fatalf("Couldn't open %s because %s", vidPath, err)
			}

			pidPath := fmt.Sprintf("%s/%s/idProduct", dirname, filename)
			pid, err := ioutil.ReadFile(pidPath)
			if err != nil {
				log.Fatalf("Couldn't open %s because %s", pidPath, err)
			}

			addressPath := fmt.Sprintf("%s/%s/devnum", dirname, filename)
			address, err := ioutil.ReadFile(addressPath)
			if err != nil {
				log.Fatalf("Couldn't open %s because %s", addressPath, err)
			}

			scannedDevices = append(
				scannedDevices,
				ScannedDevice{
					vidPid:    fmt.Sprintf("%s:%s", trim(vid), trim(pid)),
					busId:     find[1],
					addressId: trim(address),
					portPath:  find[2],
				},
			)
		}
	}
	return
}
