package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

type FacilityRepository struct {
	db *sql.DB
}

func NewFacilityRepository(db *sql.DB) *FacilityRepository {
	return &FacilityRepository{db: db}
}

func (r *FacilityRepository) List() ([]models.Facility, error) {
	rows, err := r.db.Query(`
		SELECT id, name, checkin_mode, bluetooth_beacon_id, bluetooth_beacon_range_meters,
		       kiosk_device_token, created_at, updated_at
		FROM facilities ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list facilities: %w", err)
	}
	defer rows.Close()

	var facilities []models.Facility
	for rows.Next() {
		var f models.Facility
		if err := rows.Scan(
			&f.ID, &f.Name, &f.CheckinMode, &f.BluetoothBeaconID,
			&f.BluetoothBeaconRangeM, &f.KioskDeviceToken,
			&f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan facility: %w", err)
		}
		facilities = append(facilities, f)
	}
	return facilities, rows.Err()
}

func (r *FacilityRepository) FindByID(id string) (*models.Facility, error) {
	f := &models.Facility{}
	err := r.db.QueryRow(`
		SELECT id, name, checkin_mode, bluetooth_beacon_id, bluetooth_beacon_range_meters,
		       kiosk_device_token, created_at, updated_at
		FROM facilities WHERE id = $1
	`, id).Scan(
		&f.ID, &f.Name, &f.CheckinMode, &f.BluetoothBeaconID,
		&f.BluetoothBeaconRangeM, &f.KioskDeviceToken,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find facility: %w", err)
	}
	return f, nil
}

func (r *FacilityRepository) NameExists(name string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM facilities WHERE name = $1)`, name).Scan(&exists)
	return exists, err
}

func (r *FacilityRepository) Create(f *models.Facility) error {
	return r.db.QueryRow(`
		INSERT INTO facilities (name, checkin_mode, bluetooth_beacon_id, bluetooth_beacon_range_meters, kiosk_device_token)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`, f.Name, f.CheckinMode, f.BluetoothBeaconID, f.BluetoothBeaconRangeM, f.KioskDeviceToken,
	).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt)
}

func (r *FacilityRepository) Update(f *models.Facility) error {
	return r.db.QueryRow(`
		UPDATE facilities
		SET name = $1, checkin_mode = $2, bluetooth_beacon_id = $3,
		    bluetooth_beacon_range_meters = $4, updated_at = NOW()
		WHERE id = $5
		RETURNING updated_at
	`, f.Name, f.CheckinMode, f.BluetoothBeaconID, f.BluetoothBeaconRangeM, f.ID,
	).Scan(&f.UpdatedAt)
}

func (r *FacilityRepository) UpdateKioskToken(id, token string) error {
	_, err := r.db.Exec(`
		UPDATE facilities SET kiosk_device_token = $1, updated_at = NOW() WHERE id = $2
	`, token, id)
	return err
}
