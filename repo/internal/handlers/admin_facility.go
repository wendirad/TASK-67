package handlers

import (
	"net/http"

	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

type AdminFacilityHandler struct {
	facilityService *services.FacilityService
}

func NewAdminFacilityHandler(facilityService *services.FacilityService) *AdminFacilityHandler {
	return &AdminFacilityHandler{facilityService: facilityService}
}

var validCheckinModes = map[string]bool{
	"staff_qr":           true,
	"staff_qr_bluetooth": true,
}

func (h *AdminFacilityHandler) List(c *gin.Context) {
	facilities, err := h.facilityService.ListFacilities()
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if facilities == nil {
		facilities = []models.Facility{}
	}
	Success(c, http.StatusOK, "OK", gin.H{"items": facilities})
}

func (h *AdminFacilityHandler) Create(c *gin.Context) {
	var input services.CreateFacilityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Name == "" {
		Error(c, http.StatusBadRequest, "Facility name is required")
		return
	}
	if !validCheckinModes[input.CheckinMode] {
		Error(c, http.StatusBadRequest, "Invalid checkin_mode. Must be staff_qr or staff_qr_bluetooth")
		return
	}
	if input.CheckinMode == "staff_qr_bluetooth" && (input.BluetoothBeaconID == nil || *input.BluetoothBeaconID == "") {
		Error(c, http.StatusBadRequest, "Bluetooth beacon ID is required when checkin_mode is staff_qr_bluetooth")
		return
	}

	facility, code, msg := h.facilityService.CreateFacility(input)
	if facility == nil {
		Error(c, code, msg)
		return
	}

	Created(c, "Facility created successfully", facility)
}

func (h *AdminFacilityHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var input services.UpdateFacilityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.CheckinMode != nil && !validCheckinModes[*input.CheckinMode] {
		Error(c, http.StatusBadRequest, "Invalid checkin_mode. Must be staff_qr or staff_qr_bluetooth")
		return
	}

	facility, code, msg := h.facilityService.UpdateFacility(id, input)
	if facility == nil {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Facility updated successfully", facility)
}

func (h *AdminFacilityHandler) RotateKioskToken(c *gin.Context) {
	id := c.Param("id")

	facility, code, msg := h.facilityService.RotateKioskToken(id)
	if facility == nil {
		Error(c, code, msg)
		return
	}

	Success(c, http.StatusOK, "Kiosk token rotated successfully", facility)
}
