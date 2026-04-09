package repository

import (
	"database/sql"
	"fmt"
	"time"

	"campusrec/internal/models"
)

type CatalogRepository struct {
	db *sql.DB
}

func NewCatalogRepository(db *sql.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

type CatalogQuery struct {
	Page     int
	PageSize int
	Type     string // "all", "session", "product"
	Search   string
	Category string
	Facility string
	FromDate string
	ToDate   string
	Sort     string // "relevance", "price_asc", "price_desc", "date_asc", "date_desc", "name_asc"
}

func (r *CatalogRepository) Query(q CatalogQuery) ([]models.CatalogItem, int, error) {
	includeSessions := q.Type == "all" || q.Type == "session"
	includeProducts := q.Type == "all" || q.Type == "product"

	var items []models.CatalogItem
	total := 0

	if includeSessions {
		sessionItems, sessionCount, err := r.querySessions(q)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, sessionItems...)
		total += sessionCount
	}

	if includeProducts {
		productItems, productCount, err := r.queryProducts(q)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, productItems...)
		total += productCount
	}

	sortCatalogItems(items, q.Sort)

	offset := (q.Page - 1) * q.PageSize
	end := offset + q.PageSize
	if offset >= len(items) {
		return []models.CatalogItem{}, total, nil
	}
	if end > len(items) {
		end = len(items)
	}

	return items[offset:end], total, nil
}

func (r *CatalogRepository) querySessions(q CatalogQuery) ([]models.CatalogItem, int, error) {
	baseQuery := `FROM sessions s JOIN facilities f ON s.facility_id = f.id WHERE s.status IN ('open', 'closed')`
	args := []interface{}{}
	argIdx := 1

	if q.Search != "" {
		baseQuery += fmt.Sprintf(` AND (s.title ILIKE $%d OR s.coach_name ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+q.Search+"%")
		argIdx++
	}
	if q.Facility != "" {
		baseQuery += fmt.Sprintf(` AND f.name ILIKE $%d`, argIdx)
		args = append(args, "%"+q.Facility+"%")
		argIdx++
	}
	if q.FromDate != "" {
		baseQuery += fmt.Sprintf(` AND s.start_time >= $%d`, argIdx)
		args = append(args, q.FromDate)
		argIdx++
	}
	if q.ToDate != "" {
		baseQuery += fmt.Sprintf(` AND s.start_time <= $%d`, argIdx)
		args = append(args, q.ToDate)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count catalog sessions: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT s.id, s.title, s.coach_name, f.name, s.start_time, s.end_time,
		       s.available_seats, s.total_seats, s.status,
		       s.registration_close_before_minutes
		%s ORDER BY s.start_time ASC
	`, baseQuery)

	rows, err := r.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query catalog sessions: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	var items []models.CatalogItem
	for rows.Next() {
		var id, title, facilityName, status string
		var coachName *string
		var startTime, endTime time.Time
		var availableSeats, totalSeats, closeBeforeMin int

		if err := rows.Scan(&id, &title, &coachName, &facilityName, &startTime, &endTime,
			&availableSeats, &totalSeats, &status, &closeBeforeMin); err != nil {
			return nil, 0, fmt.Errorf("scan catalog session: %w", err)
		}

		coach := ""
		if coachName != nil {
			coach = " · Coach " + *coachName
		}
		subtitle := fmt.Sprintf("%s%s · %s", facilityName, coach,
			startTime.Format("Jan 2, 15:04")+"–"+endTime.Format("15:04"))

		availability, detail := sessionAvailability(availableSeats, status, startTime, closeBeforeMin, now)

		st := startTime
		items = append(items, models.CatalogItem{
			Type:               "session",
			ID:                 id,
			Title:              title,
			Subtitle:           subtitle,
			Availability:       availability,
			AvailabilityDetail: detail,
			ImageURL:           nil,
			PriceCents:         nil,
			StartTime:          &st,
		})
	}

	return items, total, rows.Err()
}

func (r *CatalogRepository) queryProducts(q CatalogQuery) ([]models.CatalogItem, int, error) {
	baseQuery := `FROM products WHERE status = 'active'`
	args := []interface{}{}
	argIdx := 1

	if q.Search != "" {
		baseQuery += fmt.Sprintf(` AND (name ILIKE $%d OR description ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+q.Search+"%")
		argIdx++
	}
	if q.Category != "" {
		baseQuery += fmt.Sprintf(` AND category = $%d`, argIdx)
		args = append(args, q.Category)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count catalog products: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, name, category, price_cents, stock_quantity, is_shippable, image_url
		%s ORDER BY name ASC
	`, baseQuery)

	rows, err := r.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query catalog products: %w", err)
	}
	defer rows.Close()

	var items []models.CatalogItem
	for rows.Next() {
		var id, name, category string
		var priceCents, stockQuantity int
		var isShippable bool
		var imageURL *string

		if err := rows.Scan(&id, &name, &category, &priceCents, &stockQuantity, &isShippable, &imageURL); err != nil {
			return nil, 0, fmt.Errorf("scan catalog product: %w", err)
		}

		subtitle := category
		if isShippable {
			subtitle += " · Shippable"
		}

		availability, detail := productAvailability(stockQuantity)

		p := priceCents
		items = append(items, models.CatalogItem{
			Type:               "product",
			ID:                 id,
			Title:              name,
			Subtitle:           subtitle,
			Availability:       availability,
			AvailabilityDetail: detail,
			ImageURL:           imageURL,
			PriceCents:         &p,
			StartTime:          nil,
		})
	}

	return items, total, rows.Err()
}

func sessionAvailability(availableSeats int, status string, startTime time.Time, closeBeforeMin int, now time.Time) (string, string) {
	regOpen := status == "open" && now.Before(startTime.Add(-time.Duration(closeBeforeMin)*time.Minute))

	if !regOpen {
		return "closed", "Registration Closed"
	}
	if availableSeats > 5 {
		return "available", fmt.Sprintf("%d seats left", availableSeats)
	}
	if availableSeats > 0 {
		return "few_left", fmt.Sprintf("%d seats left", availableSeats)
	}
	return "full", "Full — Waitlist Available"
}

func productAvailability(stockQuantity int) (string, string) {
	if stockQuantity > 10 {
		return "in_stock", fmt.Sprintf("%d in stock", stockQuantity)
	}
	if stockQuantity > 0 {
		return "low_stock", fmt.Sprintf("%d in stock", stockQuantity)
	}
	return "out_of_stock", "Out of Stock"
}

func sortCatalogItems(items []models.CatalogItem, sortBy string) {
	switch sortBy {
	case "price_asc":
		sortByFunc(items, func(a, b models.CatalogItem) bool {
			ap := priceOrMax(a)
			bp := priceOrMax(b)
			return ap < bp
		})
	case "price_desc":
		sortByFunc(items, func(a, b models.CatalogItem) bool {
			ap := priceOrMax(a)
			bp := priceOrMax(b)
			return ap > bp
		})
	case "date_asc":
		sortByFunc(items, func(a, b models.CatalogItem) bool {
			at := timeOrMax(a)
			bt := timeOrMax(b)
			return at.Before(bt)
		})
	case "date_desc":
		sortByFunc(items, func(a, b models.CatalogItem) bool {
			at := timeOrMax(a)
			bt := timeOrMax(b)
			return at.After(bt)
		})
	case "name_asc":
		sortByFunc(items, func(a, b models.CatalogItem) bool {
			return a.Title < b.Title
		})
	default: // "relevance" or empty — sessions by date, products by name, interleaved
		// Default ordering: keep sessions first by start_time, then products by name
		// Already sorted from DB queries
	}
}

func priceOrMax(item models.CatalogItem) int {
	if item.PriceCents != nil {
		return *item.PriceCents
	}
	return 1<<31 - 1 // sessions have no price, placed after products
}

func timeOrMax(item models.CatalogItem) time.Time {
	if item.StartTime != nil {
		return *item.StartTime
	}
	return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC) // products have no date, placed after sessions
}

func sortByFunc(items []models.CatalogItem, less func(a, b models.CatalogItem) bool) {
	n := len(items)
	for i := 1; i < n; i++ {
		for j := i; j > 0 && less(items[j], items[j-1]); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}
