package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"strings"
	"sync"

	"tinygo.org/x/bluetooth"
)

const uuidService = "0000fce0-0000-1000-8000-00805f9b34fb"
const uuidCurrentReadingSimple = "f0cd1503-95da-4f4b-9ac8-aa55d312af0c"
const uuidCurrentReadingFull = "f0cd3001-95da-4f4b-9ac8-aa55d312af0c"

func mustUuidFromString(str string) bluetooth.UUID {
	uuid, err := bluetooth.ParseUUID(str)
	if err != nil {
		panic(err)
	}
	return uuid
}

type Aranet4 struct {
	adapter         *bluetooth.Adapter
	device          *bluetooth.Device
	characteristics struct {
		currentReadingSimple bluetooth.DeviceCharacteristic
		currentReadingFull   bluetooth.DeviceCharacteristic
	}
}

const ColorGreen = 1
const ColorYellow = 2
const ColorRed = 3

type CurrentReading struct {
	Temperature float64
	CO2         int
	Pressure    float64
	Humidity    float64
	Battery     int
	Quality     string
	Color       int

	Full            bool
	UpdateInterval  int
	SinceLastUpdate int
}

func NewAranet4() Aranet4 {
	return Aranet4{
		adapter: bluetooth.DefaultAdapter,
	}
}

func (a *Aranet4) Connect(strAddr string) error {
	log.Println("Enabling...")

	err := a.adapter.Enable()
	if err != nil {
		return err
	}

	address := bluetooth.Address{}
	address.Set(strings.ToUpper(strAddr))

	wg := sync.WaitGroup{}
	wg.Add(1)

	var innerErr error

	a.adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
		log.Println("Connected:", connected)

		if !connected {
			return
		}

		defer wg.Done()

		log.Println("Discovering services...")

		services, err := device.DiscoverServices([]bluetooth.UUID{
			mustUuidFromString(uuidService),
		})
		if err != nil {
			innerErr = err
			return
		}

		for _, service := range services {
			if service.UUID().String() == uuidService {
				log.Println("Discovering characteristics...")

				characteristics, err := service.DiscoverCharacteristics([]bluetooth.UUID{
					mustUuidFromString(uuidCurrentReadingSimple),
					mustUuidFromString(uuidCurrentReadingFull),
				})
				if err != nil {
					innerErr = err
					return
				}

				for _, characteristic := range characteristics {
					switch characteristic.UUID().String() {
					case uuidCurrentReadingSimple:
						a.characteristics.currentReadingSimple = characteristic
					case uuidCurrentReadingFull:
						a.characteristics.currentReadingFull = characteristic
					}
				}

				log.Println("Good to go")

				return
			}
		}
	})

	// Following the Apple accessory design guidelines, picking a connection latency of around 500ms that is a multiple
	// of 15ms (and giving the device 15ms of space). Apparently, Android 13 phone picks 510ms as the connection
	// interval with these parameters.
	log.Println("Connecting...")
	device, err := a.adapter.Connect(address, bluetooth.ConnectionParams{
		// ConnectionTimeout: bluetooth.NewDuration(10 * time.Second),
		// MinInterval:       bluetooth.NewDuration(495 * time.Millisecond),
		// MaxInterval:       bluetooth.NewDuration(510 * time.Millisecond),
		// Timeout:           bluetooth.NewDuration(5 * time.Second),
	})

	if err != nil {
		return err
	}

	log.Println("Waiting...")
	wg.Wait()

	if innerErr != nil {
		return innerErr
	}

	a.device = &device

	log.Println("CONNECTED!")

	return nil
}

func (a *Aranet4) CurrentReading(full bool) (CurrentReading, error) {
	var buffer [512]byte

	var c bluetooth.DeviceCharacteristic
	if full {
		c = a.characteristics.currentReadingFull
	} else {
		c = a.characteristics.currentReadingSimple
	}

	n, err := c.Read(buffer[:])
	if err != nil {
		return CurrentReading{}, err
	}

	isMissingData := (!full && n != 9) || (full && n != 13)
	if isMissingData {
		return CurrentReading{}, fmt.Errorf("unexpected data length: %d", n)
	}

	// fmt.Println("=> Read:", n, buffer[0:n])

	co2 := int(binary.LittleEndian.Uint16(buffer[0:2]))
	temperature := float64(binary.LittleEndian.Uint16(buffer[2:4])) / 20
	pressure := float64(binary.LittleEndian.Uint16(buffer[4:6])) / 10
	humidity := float64(buffer[6])
	battery := int(buffer[7])
	color := int(buffer[8])

	currentReading := CurrentReading{
		CO2:         co2,
		Temperature: temperature,
		Pressure:    pressure,
		Humidity:    humidity,
		Battery:     battery,
		Color:       color,
	}

	if full {
		updateInterval := int(binary.LittleEndian.Uint16(buffer[9:11]))
		sinceLastUpdate := int(binary.LittleEndian.Uint16(buffer[11:13]))

		currentReading.Full = true
		currentReading.UpdateInterval = updateInterval
		currentReading.SinceLastUpdate = sinceLastUpdate
	}

	return currentReading, nil
}

func (a *Aranet4) Disconnect() error {
	return a.device.Disconnect()
}
