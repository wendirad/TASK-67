package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

type AddressRepository struct {
	db *sql.DB
}

func NewAddressRepository(db *sql.DB) *AddressRepository {
	return &AddressRepository{db: db}
}

func (r *AddressRepository) ListByUser(userID string) ([]models.Address, error) {
	rows, err := r.db.Query(`
		SELECT id, user_id, label, recipient_name, phone, address_line1, address_line2,
		       city, province, postal_code, is_default, created_at
		FROM addresses WHERE user_id = $1
		ORDER BY is_default DESC, created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list addresses: %w", err)
	}
	defer rows.Close()

	var addresses []models.Address
	for rows.Next() {
		var a models.Address
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.Label, &a.RecipientName, &a.Phone,
			&a.AddressLine1, &a.AddressLine2, &a.City, &a.Province,
			&a.PostalCode, &a.IsDefault, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan address: %w", err)
		}
		addresses = append(addresses, a)
	}
	return addresses, rows.Err()
}

func (r *AddressRepository) FindByID(id string) (*models.Address, error) {
	a := &models.Address{}
	err := r.db.QueryRow(`
		SELECT id, user_id, label, recipient_name, phone, address_line1, address_line2,
		       city, province, postal_code, is_default, created_at
		FROM addresses WHERE id = $1
	`, id).Scan(
		&a.ID, &a.UserID, &a.Label, &a.RecipientName, &a.Phone,
		&a.AddressLine1, &a.AddressLine2, &a.City, &a.Province,
		&a.PostalCode, &a.IsDefault, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find address: %w", err)
	}
	return a, nil
}

func (r *AddressRepository) CountByUser(userID string) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM addresses WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

func (r *AddressRepository) Create(a *models.Address) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if a.IsDefault {
		if _, err := tx.Exec(`UPDATE addresses SET is_default = false WHERE user_id = $1`, a.UserID); err != nil {
			return fmt.Errorf("unset defaults: %w", err)
		}
	}

	err = tx.QueryRow(`
		INSERT INTO addresses (user_id, label, recipient_name, phone, address_line1, address_line2,
		                       city, province, postal_code, is_default)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`, a.UserID, a.Label, a.RecipientName, a.Phone, a.AddressLine1, a.AddressLine2,
		a.City, a.Province, a.PostalCode, a.IsDefault,
	).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert address: %w", err)
	}

	return tx.Commit()
}

func (r *AddressRepository) Update(a *models.Address) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if a.IsDefault {
		if _, err := tx.Exec(`UPDATE addresses SET is_default = false WHERE user_id = $1 AND id != $2`, a.UserID, a.ID); err != nil {
			return fmt.Errorf("unset defaults: %w", err)
		}
	}

	_, err = tx.Exec(`
		UPDATE addresses
		SET label = $1, recipient_name = $2, phone = $3, address_line1 = $4, address_line2 = $5,
		    city = $6, province = $7, postal_code = $8, is_default = $9
		WHERE id = $10 AND user_id = $11
	`, a.Label, a.RecipientName, a.Phone, a.AddressLine1, a.AddressLine2,
		a.City, a.Province, a.PostalCode, a.IsDefault, a.ID, a.UserID,
	)
	if err != nil {
		return fmt.Errorf("update address: %w", err)
	}

	return tx.Commit()
}

func (r *AddressRepository) Delete(id, userID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var wasDefault bool
	err = tx.QueryRow(`DELETE FROM addresses WHERE id = $1 AND user_id = $2 RETURNING is_default`, id, userID).Scan(&wasDefault)
	if err != nil {
		return fmt.Errorf("delete address: %w", err)
	}

	if wasDefault {
		_, err = tx.Exec(`
			UPDATE addresses SET is_default = true
			WHERE user_id = $1 AND id = (
				SELECT id FROM addresses WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1
			)
		`, userID)
		if err != nil {
			return fmt.Errorf("promote default: %w", err)
		}
	}

	return tx.Commit()
}

func (r *AddressRepository) SetDefault(id, userID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE addresses SET is_default = false WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("unset defaults: %w", err)
	}
	if _, err := tx.Exec(`UPDATE addresses SET is_default = true WHERE id = $1 AND user_id = $2`, id, userID); err != nil {
		return fmt.Errorf("set default: %w", err)
	}

	return tx.Commit()
}

// IsAddressInUse checks if the address is referenced by any non-terminal order.
func (r *AddressRepository) IsAddressInUse(addressID string) (bool, error) {
	var inUse bool
	err := r.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM orders
			WHERE shipping_address_id = $1
			AND status NOT IN ('delivered', 'closed', 'refunded', 'completed')
		)
	`, addressID).Scan(&inUse)
	return inUse, err
}
