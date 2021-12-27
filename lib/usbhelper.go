package lib

import (
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
)

type ScannedDevice struct {
	VidPid    string
	BusId     string
	AddressId string
	PortPath  string
	Product   string
}

func Trim(str []byte) (trimmed string) {
	return strings.Trim(string(str), "\n \r")
}

func ScanUsbDevices() (scannedDevices []ScannedDevice) {
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

			productPath := fmt.Sprintf("%s/%s/product", dirname, filename)
			product, err := ioutil.ReadFile(productPath)
			if err != nil {
				product = make([]byte, 0)
			}

			scannedDevices = append(
				scannedDevices,
				ScannedDevice{
					VidPid:    fmt.Sprintf("%s:%s", Trim(vid), Trim(pid)),
					BusId:     find[1],
					AddressId: Trim(address),
					PortPath:  find[2],
					Product:   Trim(product),
				},
			)
		}
	}
	return
}
