package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

type CartRepository struct {
	db *sql.DB
}

func NewCartRepository(db *sql.DB) *CartRepository {
	return &CartRepository{db: db}
}

// ListByUser returns all cart items for a user with joined product info.
func (r *CartRepository) ListByUser(userID string) ([]models.CartItem, error) {
	rows, err := r.db.Query(`
		SELECT ci.id, ci.product_id, ci.quantity, ci.added_at,
		       p.id, p.name, p.price_cents, p.stock_quantity, p.image_url, p.is_shippable, p.status
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.user_id = $1
		ORDER BY ci.added_at ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list cart items: %w", err)
	}
	defer rows.Close()

	var items []models.CartItem
	for rows.Next() {
		var ci models.CartItem
		var pi models.CartProductInfo
		if err := rows.Scan(
			&ci.ID, &ci.ProductID, &ci.Quantity, &ci.AddedAt,
			&pi.ID, &pi.Name, &pi.PriceCents, &pi.StockQuantity, &pi.ImageURL, &pi.IsShippable, &pi.Status,
		); err != nil {
			return nil, fmt.Errorf("scan cart item: %w", err)
		}
		ci.Product = &pi
		ci.SubtotalCents = pi.PriceCents * ci.Quantity
		items = append(items, ci)
	}
	return items, rows.Err()
}

// FindByID returns a single cart item by ID.
func (r *CartRepository) FindByID(id string) (*models.CartItem, error) {
	ci := &models.CartItem{}
	err := r.db.QueryRow(`
		SELECT id, user_id, product_id, quantity, added_at
		FROM cart_items WHERE id = $1
	`, id).Scan(&ci.ID, &ci.UserID, &ci.ProductID, &ci.Quantity, &ci.AddedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find cart item: %w", err)
	}
	return ci, nil
}

// FindByUserAndProduct returns a cart item for a specific user+product pair.
func (r *CartRepository) FindByUserAndProduct(userID, productID string) (*models.CartItem, error) {
	ci := &models.CartItem{}
	err := r.db.QueryRow(`
		SELECT id, user_id, product_id, quantity, added_at
		FROM cart_items WHERE user_id = $1 AND product_id = $2
	`, userID, productID).Scan(&ci.ID, &ci.UserID, &ci.ProductID, &ci.Quantity, &ci.AddedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find cart item by user+product: %w", err)
	}
	return ci, nil
}

// AddOrUpdate inserts a new cart item or updates quantity if the user+product pair already exists.
func (r *CartRepository) AddOrUpdate(userID, productID string, quantity int) (*models.CartItem, error) {
	ci := &models.CartItem{}
	err := r.db.QueryRow(`
		INSERT INTO cart_items (id, user_id, product_id, quantity)
		VALUES (gen_random_uuid(), $1, $2, $3)
		ON CONFLICT (user_id, product_id) DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity
		RETURNING id, user_id, product_id, quantity, added_at
	`, userID, productID, quantity).Scan(&ci.ID, &ci.UserID, &ci.ProductID, &ci.Quantity, &ci.AddedAt)
	if err != nil {
		return nil, fmt.Errorf("add/update cart item: %w", err)
	}
	return ci, nil
}

// UpdateQuantity sets the quantity for a cart item.
func (r *CartRepository) UpdateQuantity(id string, quantity int) error {
	_, err := r.db.Exec(`UPDATE cart_items SET quantity = $1 WHERE id = $2`, quantity, id)
	if err != nil {
		return fmt.Errorf("update cart quantity: %w", err)
	}
	return nil
}

// Delete removes a cart item by ID.
func (r *CartRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM cart_items WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete cart item: %w", err)
	}
	return nil
}

// DeleteByUserAndProducts removes cart items for specific product IDs belonging to a user.
func (r *CartRepository) DeleteByUserAndProducts(userID string, productIDs []string) error {
	if len(productIDs) == 0 {
		return nil
	}
	query := `DELETE FROM cart_items WHERE user_id = $1 AND product_id = ANY($2::uuid[])`
	_, err := r.db.Exec(query, userID, pqUUIDArray(productIDs))
	if err != nil {
		return fmt.Errorf("delete cart items by products: %w", err)
	}
	return nil
}

// DeleteAllByUser removes all cart items for a user.
func (r *CartRepository) DeleteAllByUser(userID string) error {
	_, err := r.db.Exec(`DELETE FROM cart_items WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete all cart items: %w", err)
	}
	return nil
}

// pqUUIDArray converts a string slice to a PostgreSQL array literal for uuid[].
func pqUUIDArray(ids []string) string {
	result := "{"
	for i, id := range ids {
		if i > 0 {
			result += ","
		}
		result += id
	}
	result += "}"
	return result
}
