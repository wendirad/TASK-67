package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

type ProductRepository struct {
	db *sql.DB
}

func NewProductRepository(db *sql.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) List(page, pageSize int, category, search, status string, minPrice, maxPrice *int, isShippable *bool) ([]models.Product, int, error) {
	baseQuery := `FROM products WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if category != "" {
		baseQuery += fmt.Sprintf(` AND category = $%d`, argIdx)
		args = append(args, category)
		argIdx++
	}
	if search != "" {
		baseQuery += fmt.Sprintf(` AND (name ILIKE $%d OR description ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		baseQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}
	if minPrice != nil {
		baseQuery += fmt.Sprintf(` AND price_cents >= $%d`, argIdx)
		args = append(args, *minPrice)
		argIdx++
	}
	if maxPrice != nil {
		baseQuery += fmt.Sprintf(` AND price_cents <= $%d`, argIdx)
		args = append(args, *maxPrice)
		argIdx++
	}
	if isShippable != nil {
		baseQuery += fmt.Sprintf(` AND is_shippable = $%d`, argIdx)
		args = append(args, *isShippable)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, name, description, category, price_cents, stock_quantity,
		       is_shippable, image_url, status, created_at, updated_at
		%s ORDER BY name ASC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.Category, &p.PriceCents,
			&p.StockQuantity, &p.IsShippable, &p.ImageURL, &p.Status,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan product: %w", err)
		}
		p.Availability = p.ComputeAvailability()
		products = append(products, p)
	}
	return products, total, rows.Err()
}

func (r *ProductRepository) FindByID(id string) (*models.Product, error) {
	p := &models.Product{}
	err := r.db.QueryRow(`
		SELECT id, name, description, category, price_cents, stock_quantity,
		       is_shippable, image_url, status, created_at, updated_at
		FROM products WHERE id = $1
	`, id).Scan(
		&p.ID, &p.Name, &p.Description, &p.Category, &p.PriceCents,
		&p.StockQuantity, &p.IsShippable, &p.ImageURL, &p.Status,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find product: %w", err)
	}
	p.Availability = p.ComputeAvailability()
	return p, nil
}
