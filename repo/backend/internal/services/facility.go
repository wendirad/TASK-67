package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type FacilityService struct {
	facilityRepo *repository.FacilityRepository
	jwtSecret    string
}

func NewFacilityService(facilityRepo *repository.FacilityRepository, jwtSecret string) *FacilityService {
	return &FacilityService{facilityRepo: facilityRepo, jwtSecret: jwtSecret}
}

func (s *FacilityService) ListFacilities() ([]models.Facility, error) {
	return s.facilityRepo.List()
}

type CreateFacilityInput struct {
	Name                    string  `json:"name"`
	CheckinMode             string  `json:"checkin_mode"`
	BluetoothBeaconID       *string `json:"bluetooth_beacon_id"`
	BluetoothBeaconRangeM   *int    `json:"bluetooth_beacon_range_meters"`
}

func (s *FacilityService) CreateFacility(input CreateFacilityInput) (*models.Facility, int, string) {
	exists, err := s.facilityRepo.NameExists(input.Name)
	if err != nil {
		log.Printf("Error checking facility name: %v", err)
		return nil, 500, "Internal server error"
	}
	if exists {
		return nil, 409, "Facility name already exists"
	}

	token := s.generateKioskToken(input.Name)

	f := &models.Facility{
		Name:                    input.Name,
		CheckinMode:             input.CheckinMode,
		BluetoothBeaconID:       input.BluetoothBeaconID,
		BluetoothBeaconRangeM:   input.BluetoothBeaconRangeM,
		KioskDeviceToken:        &token,
	}

	if err := s.facilityRepo.Create(f); err != nil {
		log.Printf("Error creating facility: %v", err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Facility created: %s", f.Name)
	return f, 201, ""
}

type UpdateFacilityInput struct {
	Name                    *string `json:"name"`
	CheckinMode             *string `json:"checkin_mode"`
	BluetoothBeaconID       *string `json:"bluetooth_beacon_id"`
	BluetoothBeaconRangeM   *int    `json:"bluetooth_beacon_range_meters"`
}

func (s *FacilityService) UpdateFacility(id string, input UpdateFacilityInput) (*models.Facility, int, string) {
	f, err := s.facilityRepo.FindByID(id)
	if err != nil {
		log.Printf("Error finding facility %s: %v", id, err)
		return nil, 500, "Internal server error"
	}
	if f == nil {
		return nil, 404, "Facility not found"
	}

	if input.Name != nil {
		f.Name = *input.Name
	}
	if input.CheckinMode != nil {
		f.CheckinMode = *input.CheckinMode
	}
	if input.BluetoothBeaconID != nil {
		f.BluetoothBeaconID = input.BluetoothBeaconID
	}
	if input.BluetoothBeaconRangeM != nil {
		f.BluetoothBeaconRangeM = input.BluetoothBeaconRangeM
	}

	if f.CheckinMode == "staff_qr_bluetooth" && (f.BluetoothBeaconID == nil || *f.BluetoothBeaconID == "") {
		return nil, 400, "Bluetooth beacon ID is required when checkin_mode is staff_qr_bluetooth"
	}

	if err := s.facilityRepo.Update(f); err != nil {
		log.Printf("Error updating facility %s: %v", id, err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Facility updated: %s", f.Name)
	return f, 200, ""
}

func (s *FacilityService) RotateKioskToken(id string) (*models.Facility, int, string) {
	f, err := s.facilityRepo.FindByID(id)
	if err != nil {
		log.Printf("Error finding facility %s: %v", id, err)
		return nil, 500, "Internal server error"
	}
	if f == nil {
		return nil, 404, "Facility not found"
	}

	token := s.generateKioskToken(f.Name)
	if err := s.facilityRepo.UpdateKioskToken(id, token); err != nil {
		log.Printf("Error rotating kiosk token for facility %s: %v", id, err)
		return nil, 500, "Internal server error"
	}
	f.KioskDeviceToken = &token

	log.Printf("Kiosk token rotated for facility %s", f.Name)
	return f, 200, ""
}

func (s *FacilityService) generateKioskToken(facilityName string) string {
	mac := hmac.New(sha256.New, []byte(s.jwtSecret))
	mac.Write([]byte(fmt.Sprintf("kiosk:%s:%d", facilityName, time.Now().UnixNano())))
	return hex.EncodeToString(mac.Sum(nil))
}
