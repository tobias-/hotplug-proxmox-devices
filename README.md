# Hotplug USB Devices for QEMU

This is a program to automate the disconnecting/connecting of usb devices to different VMs. I use it to have a true "two-in-one" computer where both Windows and Linux are running at the same time and both have dedicated graphics cards, a KVM-switch and one keyboard/mouse.

Other possible uses is switching USB hardware between different VMs for e.g. testing.

## Hardware installation

The idea is that you use a KVM-switch.

You could probably just physically pull the usb cord from one usb-port on the computer and put it into another, but make sure to remember which port on the computer goes to which VM.

### Using KVM-switch *REALLY* recommended

Connect the KVM-switch output to two ports on the computer. Connect all the devices to the KVM-switch inputs. In my case I have a builtin usb-hub in the KVM-switch so I didn't need a usb-hub.

## Finding the port paths

There is no really easy way of doing this. Your best bet is probably to use `qm monitor 100` (using 100 here, but it doesn't matter which VM) and then running `info usbhost` in that interactive program.

Assuming you've on the KVM switch selected that you want the devices on VM 100 and run the `info usbhost`, what you get is a list entries similar to this:

```
  Bus 3, Addr 45, Port 6.1.4, Speed 12 Mb/s
    Class ff: USB device 1a86:7523, USB Serial
```

The format is `<VMID>:<Bus>-<Port>`, i.e. `100:3-6.1.4`. Switch the KVM-switch to the next VM, and run `info usbhost` again to get the information necessary for that VM's entry.

## Installation

### Prerequisites

* Golang installed. It's quite easy to do via https://golang.org/doc/install

### Building

```shell
git clone https://github.com/tobias-/hotplug-proxmox-devices
cd hotplug-proxmox-devices
go build -o hotplugDevices *.go
cp hotplugDevices /usr/local/sbin/
```

### Setting up script

This is unfortunately relatively brittle, as there is no good way of getting guaranteed stable USBs afaik, but it should be stable unless you both change the amount of USB connected and reboot the host.

I've put the following script into `/usr/local/sbin/triggerKvmUpdate.sh`

```shell
#!/bin/bash -eux
# Allow the /sys/ directory to settle before running this.
sleep 2
/usr/local/sbin/hotplugDevices -d 1a86:7523 -p 100:5-2.4 -p 101:3-6.1.4 -t 0a81:0103
```

(Don't forget to `chmod u+x /usr/local/sbin/triggerKvmUpdate.sh`)

`-d 1a86:7523` is the detection device. The bus/port path of this device is the detector for which VM should get the target devices. This could probably be the same as one of the target devices, but I have another for it.

`-p 100:5-2.4` and `-p 101:3-6.1.4` are the different paths to the detection device. E.g. if it's found on bus `5` with port path `2.4`, all target devices will be disconnected for other VMs and connected to VM `100`.

`-t 0a81:0103` is a target device. May be specified multiple times to switch multiple devices.

### Setting up udev

This is my udev script put into `/etc/udev/rules.d/10-kvm.rules`

```udev
ATTRS{idVendor}=="1a86", ATTRS{idProduct}=="7523", RUN+="/usr/local/sbin/updateKvm.sh"
```

When the kernel detects a connection of the detection device, it will run the `updateKvm.sh` script, thus moving the device
