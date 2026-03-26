//go:build linux
// +build linux

package evdev

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

/*

#include <linux/input.h>
#include <stdlib.h>
*/
import "C"

// A Linux input device from which events can be read.
type InputDevice struct {
	Fn string // path to input device (devnode)

	Name string   // device name
	Phys string   // physical topology of device
	File *os.File // an open file handle to the input device

	Bustype uint16 // bus type identifier
	Vendor  uint16 // vendor identifier
	Product uint16 // product identifier
	Version uint16 // version identifier

	EvdevVersion int // evdev protocol version

	//Capabilities     map[CapabilityType][]CapabilityCode // supported event types and codes.
	Capabilities map[string]map[int]string
	AbsInfo      map[string]AbsInfo
}

// Open an evdev input device.
func Open(devnode string) (*InputDevice, error) {
	f, err := os.OpenFile(devnode, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	dev := InputDevice{}
	dev.Fn = devnode
	dev.File = f

	err = dev.set_device_info()
	if err != nil {
		return nil, err
	}
	err = dev.set_device_capabilities()
	if err != nil {
		return nil, err
	}

	return &dev, nil
}

// Read and return a slice of input events from device.
func (dev *InputDevice) Read() ([]InputEvent, error) {
	events := make([]InputEvent, 16)
	buffer := make([]byte, eventsize*16)

	_, err := dev.File.Read(buffer)
	if err != nil {
		return events, err
	}

	b := bytes.NewBuffer(buffer)
	err = binary.Read(b, binary.LittleEndian, &events)
	if err != nil {
		return events, err
	}

	// remove trailing structures
	for i := range events {
		if events[i].Time.Sec == 0 {
			events = append(events[:i])
			break
		}
	}

	return events, err
}

// Read and return a single input event.
func (dev *InputDevice) ReadOne() (*InputEvent, error) {
	event := InputEvent{}
	buffer := make([]byte, eventsize)

	_, err := dev.File.Read(buffer)
	if err != nil {
		return &event, err
	}

	b := bytes.NewBuffer(buffer)
	err = binary.Read(b, binary.LittleEndian, &event)
	if err != nil {
		return &event, err
	}

	return &event, err
}

// Write a single input event.
func (dev *InputDevice) WriteOne(event *InputEvent) error {
	b := new(bytes.Buffer)
	err := binary.Write(b, binary.LittleEndian, event)
	if err != nil {
		return err
	}

	_, err = dev.File.Write(b.Bytes())
	return err
}

// Get a useful description for an input device. Example:
//
//	InputDevice /dev/input/event3 (fd 3)
//	  name Logitech USB Laser Mouse
//	  phys usb-0000:00:12.0-2/input0
//	  bus 0x3, vendor 0x46d, product 0xc069, version 0x110
//	  events EV_KEY 1, EV_SYN 0, EV_REL 2, EV_MSC 4
func (dev *InputDevice) String() string {

	evtypes := make([]string, 0)

	for evname, evtype := range dev.Capabilities {
		evtypes = append(evtypes, fmt.Sprintf("%s %v", evname, evtype))
	}

	evtypes_s := strings.Join(evtypes, ", ")

	return fmt.Sprintf(
		"InputDevice %s (fd %d)\n"+
			"  name %s\n"+
			"  phys %s\n"+
			"  bus 0x%04x, vendor 0x%04x, product 0x%04x, version 0x%04x\n"+
			"  events %s",
		dev.Fn, dev.File.Fd(), dev.Name, dev.Phys, dev.Bustype,
		dev.Vendor, dev.Product, dev.Version, evtypes_s)
}

// Gets the event types and event codes that the input device supports.
func (dev *InputDevice) set_device_capabilities() error {
	// Capabilities is a map of supported event types to lists of
	// events e.g: {1: [272, 273, 274, 275], 2: [0, 1, 6, 8]}
	// capabilities := make(map[int][]int)

	dev.Capabilities = make(map[string]map[int]string)

	//capabilities := make(map[CapabilityType][]CapabilityCode)

	dev.AbsInfo = make(map[string]AbsInfo)

	evbits := new([(EV_MAX + 1) / 8]byte)
	codebits := new([(KEY_MAX + 1) / 8]byte)
	absbits := new([(ABS_MAX + 1) / 8]byte)
	// absbits  := new([6]byte)

	err := ioctl(dev.File.Fd(), uintptr(EVIOCGBIT(0, EV_MAX)), unsafe.Pointer(evbits))
	if err != 0 {
		return err
	}

	// Build a map of the device's capabilities
	for evtype := 0; evtype < EV_MAX; evtype++ {
		if evbits[evtype/8]&(1<<uint(evtype%8)) != 0 {
			//eventcodes := make([]CapabilityCode, 0)
			flatteneventcodes := make(map[int]string)

			err = ioctl(dev.File.Fd(), uintptr(EVIOCGBIT(evtype, KEY_MAX)), unsafe.Pointer(codebits))
			if err != 0 {
				// ignore invalid capabilities such as EV_REP for some devices
				if err == syscall.EINVAL {
					continue
				}

				return err
			}

			for evcode := 0; evcode < KEY_MAX; evcode++ {
				if codebits[evcode/8]&(1<<uint(evcode%8)) != 0 {
					//c := CapabilityCode{evcode, ByEventType[evtype][evcode]}
					//eventcodes = append(eventcodes, c)
					flatteneventcodes[evcode] = ByEventType[evtype][evcode]
				}
			}

			// capabilities[EV_KEY] = [KEY_A, KEY_B, KEY_C, ...]
			/*
				key := CapabilityType{evtype, EV[evtype]}
				capabilities[key] = eventcodes*/
			dev.Capabilities[EV[evtype]] = flatteneventcodes

		}
	}

	err = ioctl(dev.File.Fd(), uintptr(EVIOCGBIT(EV_ABS, (ABS_MAX+1)/8)), unsafe.Pointer(absbits))

	if err != 0 {
		return err
	}

	for abstype := 0; abstype < ABS_MAX; abstype++ {

		// Skip hats
		if abstype == ABS_HAT0X {
			abstype = ABS_HAT3Y
			continue
		}

		if absbits[abstype/8]&(1<<uint(abstype%8)) != 0 {

			absinfo := C.malloc(C.sizeof_struct_input_absinfo)

			if ioctl(dev.File.Fd(), uintptr(EVIOCGABS(abstype)), unsafe.Pointer(absinfo)) < 0 {
				continue
			} else {

				var a AbsInfo
				a.Unpack(unsafe.Pointer(absinfo))
				//key := CapabilityType{abstype, ABS[abstype]}
				dev.AbsInfo[ABS[abstype]] = a

			}

			C.free(absinfo)

		}

	}

	return nil
}

// An all-in-one function for describing an input device.
func (dev *InputDevice) set_device_info() error {
	info := device_info{}

	name := new([MAX_NAME_SIZE]byte)
	phys := new([MAX_NAME_SIZE]byte)

	err := ioctl(dev.File.Fd(), uintptr(EVIOCGID), unsafe.Pointer(&info))
	if err != 0 {
		return err
	}

	err = ioctl(dev.File.Fd(), uintptr(EVIOCGNAME), unsafe.Pointer(name))
	if err != 0 {
		return err
	}

	// it's ok if the topology info is not available
	ioctl(dev.File.Fd(), uintptr(EVIOCGPHYS), unsafe.Pointer(phys))

	dev.Name = bytes_to_string(name)
	dev.Phys = bytes_to_string(phys)

	dev.Vendor = info.vendor
	dev.Bustype = info.bustype
	dev.Product = info.product
	dev.Version = info.version

	ev_version := new(int)
	err = ioctl(dev.File.Fd(), uintptr(EVIOCGVERSION), unsafe.Pointer(ev_version))
	if err != 0 {
		return err
	}
	dev.EvdevVersion = *ev_version

	return nil
}

// Get repeat rate as a two element array.
//
//	[0] repeat rate in characters per second
//	[1] amount of time that a key must be depressed before it will start
//	    to repeat (in milliseconds)
func (dev *InputDevice) GetRepeatRate() *[2]uint {
	repeat_delay := new([2]uint)
	ioctl(dev.File.Fd(), uintptr(EVIOCGREP), unsafe.Pointer(repeat_delay))

	return repeat_delay
}

// Set repeat rate and delay.
func (dev *InputDevice) SetRepeatRate(repeat, delay uint) {
	repeat_delay := new([2]uint)
	repeat_delay[0], repeat_delay[1] = repeat, delay
	ioctl(dev.File.Fd(), uintptr(EVIOCSREP), unsafe.Pointer(repeat_delay))
}

// Grab the input device exclusively.
func (dev *InputDevice) Grab() error {
	grab := int(1)
	if err := ioctl(dev.File.Fd(), uintptr(EVIOCGRAB), unsafe.Pointer(&grab)); err != 0 {
		return err
	}

	return nil
}

// Release a grabbed input device.
func (dev *InputDevice) Release() error {
	if err := ioctl(dev.File.Fd(), uintptr(EVIOCGRAB), unsafe.Pointer(nil)); err != 0 {
		return err
	}

	return nil
}

type CapabilityType struct {
	Type int
	Name string
}

type CapabilityCode struct {
	Code int
	Name string
}

type AbsInfo struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

func (a *AbsInfo) Unpack(data unsafe.Pointer) {

	cdata := C.GoBytes(data, C.sizeof_struct_input_absinfo)
	buf := bytes.NewBuffer(cdata)

	binary.Read(buf, binary.LittleEndian, &a.Value)
	binary.Read(buf, binary.LittleEndian, &a.Minimum)
	binary.Read(buf, binary.LittleEndian, &a.Maximum)
	binary.Read(buf, binary.LittleEndian, &a.Fuzz)
	binary.Read(buf, binary.LittleEndian, &a.Flat)
	binary.Read(buf, binary.LittleEndian, &a.Resolution)
}

// Corresponds to the input_id struct.
type device_info struct {
	bustype, vendor, product, version uint16
}

// Return the keys of a map as a slice (dict.keys())
func keys(cap *map[int][]int) []int {
	slice := make([]int, 0)

	for key := range *cap {
		slice = append(slice, key)
	}

	return slice
}

// Determine if a path exist and is a character input device.
func IsInputDevice(path string) bool {
	fi, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	}

	m := fi.Mode()
	if m&os.ModeCharDevice == 0 {
		return false
	}

	return true
}

// Return a list of accessible input device names matched by
// deviceglob (default '/dev/input/event*').
func ListInputDevicePaths(device_glob string) ([]string, error) {
	paths, err := filepath.Glob(device_glob)

	if err != nil {
		return nil, err
	}

	devices := make([]string, 0)
	for _, path := range paths {
		if IsInputDevice(path) {
			devices = append(devices, path)
		}
	}

	return devices, nil
}

// Return a list of accessible input devices matched by deviceglob
// (default '/dev/input/event/*').
func ListInputDevices(device_glob_arg ...string) ([]*InputDevice, error) {
	device_glob := "/dev/input/event*"
	if len(device_glob_arg) > 0 {
		device_glob = device_glob_arg[0]
	}

	fns, _ := ListInputDevicePaths(device_glob)
	devices := make([]*InputDevice, 0)

	for i := range fns {
		dev, err := Open(fns[i])
		if err == nil {
			devices = append(devices, dev)
		}
	}

	return devices, nil
}

func bytes_to_string(b *[MAX_NAME_SIZE]byte) string {
	idx := bytes.IndexByte(b[:], 0)
	return string(b[:idx])
}
