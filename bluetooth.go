package main

import (
	"github.com/muka/go-bluetooth/bluez/profile/gatt"
)

func (dev *Device) devCharByUUID(id string) (*gatt.GattCharacteristic1, error) {
	return dev.dev.GetCharByUUID(id)
}
