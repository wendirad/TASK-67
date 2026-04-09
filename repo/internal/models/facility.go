package models

import "time"

type Facility struct {
	ID                       string    `json:"id"`
	Name                     string    `json:"name"`
	CheckinMode              string    `json:"checkin_mode"`
	BluetoothBeaconID        *string   `json:"bluetooth_beacon_id"`
	BluetoothBeaconRangeM    *int      `json:"bluetooth_beacon_range_meters"`
	KioskDeviceToken         *string   `json:"kiosk_device_token,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}
