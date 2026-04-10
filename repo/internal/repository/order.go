package repository

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"campusrec/internal/models"
)

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// CreateOrder creates an order with items and payment record in a single transaction.
// It atomically deducts stock and snapshots the shipping address.
// paymentTimeoutMinutes controls the payment deadline; callers should resolve
// this from the canary-aware config before calling.
func (r *OrderRepository) CreateOrder(
	userID string,
	items []models.CreateOrderItem,
	products map[string]*models.Product,
	address *models.Address,
	paymentTimeoutMinutes int,
) (*models.Order, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Deduct stock for each product atomically
	for _, item := range items {
		result, err := tx.Exec(`
			UPDATE products SET stock_quantity = stock_quantity - $1, updated_at = NOW()
			WHERE id = $2 AND stock_quantity >= $1 AND status = 'active'
		`, item.Quantity, item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("deduct stock for product %s: %w", item.ProductID, err)
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			p := products[item.ProductID]
			return nil, fmt.Errorf("insufficient stock for product: %s", p.Name)
		}
	}

	// Compute total
	totalCents := models.OrderTotalCents(items, products)

	// Set payment deadline from canary-aware config
	paymentDeadline := time.Now().Add(time.Duration(paymentTimeoutMinutes) * time.Minute)

	// Create order
	order := &models.Order{
		UserID:          userID,
		Status:          "pending_payment",
		TotalCents:      totalCents,
		PaymentDeadline: &paymentDeadline,
	}

	// Snapshot shipping address if provided
	if address != nil {
		order.ShippingAddressID = &address.ID
		order.ShipToRecipient = &address.RecipientName
		order.ShipToPhone = &address.Phone
		order.ShipToLine1 = &address.AddressLine1
		order.ShipToLine2 = address.AddressLine2
		order.ShipToCity = &address.City
		order.ShipToProvince = &address.Province
		order.ShipToPostalCode = &address.PostalCode
	}

	// Insert order with retry on the unlikely event of an order number collision
	var insertErr error
	for attempt := 0; attempt < 3; attempt++ {
		order.OrderNumber = generateOrderNumber()
		insertErr = tx.QueryRow(`
			INSERT INTO orders (id, order_number, user_id, status, total_cents,
			    shipping_address_id, ship_to_recipient, ship_to_phone,
			    ship_to_line1, ship_to_line2, ship_to_city, ship_to_province, ship_to_postal_code,
			    payment_deadline)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING id, created_at, updated_at
		`, order.OrderNumber, order.UserID, order.Status, order.TotalCents,
			order.ShippingAddressID, order.ShipToRecipient, order.ShipToPhone,
			order.ShipToLine1, order.ShipToLine2, order.ShipToCity, order.ShipToProvince, order.ShipToPostalCode,
			order.PaymentDeadline,
		).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)
		if insertErr == nil {
			break
		}
		if !strings.Contains(insertErr.Error(), "duplicate key") {
			return nil, fmt.Errorf("insert order: %w", insertErr)
		}
	}
	if insertErr != nil {
		return nil, fmt.Errorf("insert order after retries: %w", insertErr)
	}

	// Create order items
	order.Items = make([]models.OrderItem, 0, len(items))
	for _, item := range items {
		p := products[item.ProductID]
		oi := models.OrderItem{
			ProductID:      item.ProductID,
			ProductName:    p.Name,
			Quantity:       item.Quantity,
			UnitPriceCents: p.PriceCents,
			TotalCents:     p.PriceCents * item.Quantity,
		}
		err = tx.QueryRow(`
			INSERT INTO order_items (id, order_id, product_id, product_name, quantity, unit_price_cents, total_cents)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
			RETURNING id
		`, order.ID, oi.ProductID, oi.ProductName, oi.Quantity, oi.UnitPriceCents, oi.TotalCents,
		).Scan(&oi.ID)
		if err != nil {
			return nil, fmt.Errorf("insert order item: %w", err)
		}
		order.Items = append(order.Items, oi)
	}

	// Create payment record with prepay data for QR code rendering
	prepayData := fmt.Sprintf(
		`{"appid":"wx_campusrec","mch_id":"campusrec_001","prepay_id":"PREPAY-%s","out_trade_no":"%s","total_fee":%d,"currency":"CNY","trade_type":"NATIVE"}`,
		order.ID[:8], order.OrderNumber, totalCents,
	)
	payment := &models.Payment{
		PaymentMethod:    "wechat_pay",
		AmountCents:      totalCents,
		Status:           "pending",
		WeChatPrepayData: &prepayData,
	}
	err = tx.QueryRow(`
		INSERT INTO payments (id, order_id, payment_method, amount_cents, status, wechat_prepay_data)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`, order.ID, payment.PaymentMethod, payment.AmountCents, payment.Status, payment.WeChatPrepayData,
	).Scan(&payment.ID, &payment.CreatedAt, &payment.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert payment: %w", err)
	}
	order.Payment = payment

	// Create shipping record if any item is shippable
	hasShippable := false
	for _, item := range items {
		if products[item.ProductID].IsShippable {
			hasShippable = true
			break
		}
	}
	if hasShippable {
		_, err = tx.Exec(`
			INSERT INTO shipping_records (order_id, status)
			VALUES ($1, 'pending')
		`, order.ID)
		if err != nil {
			return nil, fmt.Errorf("create shipping record: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit order: %w", err)
	}

	return order, nil
}

// FindByID returns a full order with items and payment.
func (r *OrderRepository) FindByID(id string) (*models.Order, error) {
	o := &models.Order{}
	err := r.db.QueryRow(`
		SELECT id, order_number, user_id, status, total_cents,
		       shipping_address_id, ship_to_recipient, ship_to_phone,
		       ship_to_line1, ship_to_line2, ship_to_city, ship_to_province, ship_to_postal_code,
		       payment_deadline, paid_at, closed_at, close_reason, notes,
		       created_at, updated_at
		FROM orders WHERE id = $1
	`, id).Scan(
		&o.ID, &o.OrderNumber, &o.UserID, &o.Status, &o.TotalCents,
		&o.ShippingAddressID, &o.ShipToRecipient, &o.ShipToPhone,
		&o.ShipToLine1, &o.ShipToLine2, &o.ShipToCity, &o.ShipToProvince, &o.ShipToPostalCode,
		&o.PaymentDeadline, &o.PaidAt, &o.ClosedAt, &o.CloseReason, &o.Notes,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find order: %w", err)
	}

	// Load items
	items, err := r.loadOrderItems(o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items

	// Load payment
	payment, err := r.loadPayment(o.ID)
	if err != nil {
		return nil, err
	}
	o.Payment = payment

	return o, nil
}

// ListByUser returns paginated orders for a member.
func (r *OrderRepository) ListByUser(userID string, page, pageSize int, status string) ([]models.Order, int, error) {
	baseQuery := `FROM orders WHERE user_id = $1`
	args := []interface{}{userID}
	argIdx := 2

	if status != "" {
		baseQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, order_number, user_id, status, total_cents,
		       shipping_address_id, ship_to_recipient, ship_to_phone,
		       ship_to_line1, ship_to_line2, ship_to_city, ship_to_province, ship_to_postal_code,
		       payment_deadline, paid_at, closed_at, close_reason, notes,
		       created_at, updated_at
		%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	orders, err := r.scanOrders(selectQuery, args)
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// ListAll returns paginated orders for admin/staff.
func (r *OrderRepository) ListAll(page, pageSize int, status, userID string) ([]models.Order, int, error) {
	baseQuery := `FROM orders o LEFT JOIN users u ON u.id = o.user_id WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if status != "" {
		baseQuery += fmt.Sprintf(` AND o.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}
	if userID != "" {
		baseQuery += fmt.Sprintf(` AND o.user_id = $%d`, argIdx)
		args = append(args, userID)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT o.id, o.order_number, o.user_id, o.status, o.total_cents,
		       o.shipping_address_id, o.ship_to_recipient, o.ship_to_phone,
		       o.ship_to_line1, o.ship_to_line2, o.ship_to_city, o.ship_to_province, o.ship_to_postal_code,
		       o.payment_deadline, o.paid_at, o.closed_at, o.close_reason, o.notes,
		       o.created_at, o.updated_at, u.username, u.display_name
		%s ORDER BY o.created_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list all orders: %w", err)
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(
			&o.ID, &o.OrderNumber, &o.UserID, &o.Status, &o.TotalCents,
			&o.ShippingAddressID, &o.ShipToRecipient, &o.ShipToPhone,
			&o.ShipToLine1, &o.ShipToLine2, &o.ShipToCity, &o.ShipToProvince, &o.ShipToPostalCode,
			&o.PaymentDeadline, &o.PaidAt, &o.ClosedAt, &o.CloseReason, &o.Notes,
			&o.CreatedAt, &o.UpdatedAt, &o.Username, &o.DisplayName,
		); err != nil {
			return nil, 0, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

// ConfirmPayment atomically transitions a payment to confirmed and order to paid.
// Returns (alreadyProcessed, error). alreadyProcessed=true means the transactionID
// was already recorded (idempotent success).
func (r *OrderRepository) ConfirmPayment(orderID, transactionID, callbackSignature string) (bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Check for idempotency: if this transaction_id already exists, it's a duplicate callback
	var existingPaymentID string
	err = tx.QueryRow(`
		SELECT id FROM payments WHERE transaction_id = $1
	`, transactionID).Scan(&existingPaymentID)
	if err == nil {
		// Already processed
		return true, nil
	}
	if err != sql.ErrNoRows {
		return false, fmt.Errorf("check idempotency: %w", err)
	}

	// Update payment: pending → confirmed
	result, err := tx.Exec(`
		UPDATE payments SET status = 'confirmed', transaction_id = $1,
		    callback_signature = $2, callback_received_at = NOW(), updated_at = NOW()
		WHERE order_id = $3 AND status = 'pending'
	`, transactionID, callbackSignature, orderID)
	if err != nil {
		return false, fmt.Errorf("confirm payment: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return false, fmt.Errorf("no pending payment found for order")
	}

	// Update order: pending_payment → paid
	result, err = tx.Exec(`
		UPDATE orders SET status = 'paid', paid_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'pending_payment'
	`, orderID)
	if err != nil {
		return false, fmt.Errorf("update order to paid: %w", err)
	}
	affected, _ = result.RowsAffected()
	if affected == 0 {
		return false, fmt.Errorf("order is not in pending_payment state")
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit payment confirmation: %w", err)
	}
	return false, nil
}

// FindOrderByNumber looks up an order by its order_number.
func (r *OrderRepository) FindOrderByNumber(orderNumber string) (*Order, error) {
	var orderID string
	var totalCents int
	var status string
	err := r.db.QueryRow(`
		SELECT id, total_cents, status FROM orders WHERE order_number = $1
	`, orderNumber).Scan(&orderID, &totalCents, &status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find order by number: %w", err)
	}
	return &Order{ID: orderID, TotalCents: totalCents, Status: status}, nil
}

// Order is a lightweight struct for internal lookups.
type Order struct {
	ID         string
	TotalCents int
	Status     string
}

// CancelOrder cancels an order and restores stock atomically.
func (r *OrderRepository) CancelOrder(orderID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Transition status
	result, err := tx.Exec(`
		UPDATE orders SET status = 'closed', closed_at = NOW(),
		    close_reason = 'Canceled by user', updated_at = NOW()
		WHERE id = $1 AND status = 'pending_payment'
	`, orderID)
	if err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("order cannot be canceled in current state")
	}

	// Restore stock
	if err := r.restoreStock(tx, orderID); err != nil {
		return err
	}

	// Cancel payment
	_, err = tx.Exec(`
		UPDATE payments SET status = 'failed', updated_at = NOW()
		WHERE order_id = $1 AND status = 'pending'
	`, orderID)
	if err != nil {
		return fmt.Errorf("cancel payment: %w", err)
	}

	return tx.Commit()
}

// RefundOrder processes a refund for a paid order.
func (r *OrderRepository) RefundOrder(orderID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Transition to refunded
	result, err := tx.Exec(`
		UPDATE orders SET status = 'refunded', updated_at = NOW()
		WHERE id = $1 AND status IN ('paid', 'processing', 'shipped', 'delivered', 'completed')
	`, orderID)
	if err != nil {
		return fmt.Errorf("refund order: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("order cannot be refunded in current state")
	}

	// Create refund payment record
	refundID := generateRefundID()
	_, err = tx.Exec(`
		UPDATE payments SET status = 'refunded', refund_id = $1, refunded_at = NOW(), updated_at = NOW()
		WHERE order_id = $2 AND status = 'confirmed'
	`, refundID, orderID)
	if err != nil {
		return fmt.Errorf("create refund: %w", err)
	}

	// Restore stock
	if err := r.restoreStock(tx, orderID); err != nil {
		return err
	}

	return tx.Commit()
}

// CloseExpiredOrder closes a single expired order and restores stock.
// Returns true if the order was actually closed (not already closed by another worker).
func (r *OrderRepository) CloseExpiredOrder(orderID string) (bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE orders SET status = 'closed', closed_at = NOW(),
		    close_reason = 'Payment timeout - deadline exceeded',
		    updated_at = NOW()
		WHERE id = $1 AND status = 'pending_payment'
	`, orderID)
	if err != nil {
		return false, fmt.Errorf("close expired order: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return false, nil
	}

	if err := r.restoreStock(tx, orderID); err != nil {
		return false, err
	}

	// Cancel payment
	_, err = tx.Exec(`
		UPDATE payments SET status = 'failed', updated_at = NOW()
		WHERE order_id = $1 AND status = 'pending'
	`, orderID)
	if err != nil {
		return false, fmt.Errorf("cancel payment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit close: %w", err)
	}
	return true, nil
}

// FindExpiredOrderIDs returns IDs of orders past their payment deadline.
func (r *OrderRepository) FindExpiredOrderIDs() ([]string, error) {
	rows, err := r.db.Query(`
		SELECT id FROM orders
		WHERE status = 'pending_payment' AND payment_deadline < NOW()
		FOR UPDATE SKIP LOCKED
	`)
	if err != nil {
		return nil, fmt.Errorf("find expired orders: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan expired order id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *OrderRepository) restoreStock(tx *sql.Tx, orderID string) error {
	_, err := tx.Exec(`
		UPDATE products p
		SET stock_quantity = p.stock_quantity + oi.quantity, updated_at = NOW()
		FROM order_items oi
		WHERE oi.order_id = $1 AND oi.product_id = p.id
	`, orderID)
	if err != nil {
		return fmt.Errorf("restore stock: %w", err)
	}
	return nil
}

func (r *OrderRepository) scanOrders(query string, args []interface{}) ([]models.Order, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(
			&o.ID, &o.OrderNumber, &o.UserID, &o.Status, &o.TotalCents,
			&o.ShippingAddressID, &o.ShipToRecipient, &o.ShipToPhone,
			&o.ShipToLine1, &o.ShipToLine2, &o.ShipToCity, &o.ShipToProvince, &o.ShipToPostalCode,
			&o.PaymentDeadline, &o.PaidAt, &o.ClosedAt, &o.CloseReason, &o.Notes,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *OrderRepository) loadOrderItems(orderID string) ([]models.OrderItem, error) {
	rows, err := r.db.Query(`
		SELECT id, order_id, product_id, product_name, quantity, unit_price_cents, total_cents
		FROM order_items WHERE order_id = $1
		ORDER BY product_name ASC
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("load order items: %w", err)
	}
	defer rows.Close()

	var items []models.OrderItem
	for rows.Next() {
		var oi models.OrderItem
		if err := rows.Scan(
			&oi.ID, &oi.OrderID, &oi.ProductID, &oi.ProductName,
			&oi.Quantity, &oi.UnitPriceCents, &oi.TotalCents,
		); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		items = append(items, oi)
	}
	return items, rows.Err()
}

func (r *OrderRepository) loadPayment(orderID string) (*models.Payment, error) {
	p := &models.Payment{}
	err := r.db.QueryRow(`
		SELECT id, order_id, payment_method, amount_cents, status,
		       transaction_id, wechat_prepay_data, callback_signature,
		       callback_received_at, refund_id, refunded_at,
		       created_at, updated_at
		FROM payments WHERE order_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, orderID).Scan(
		&p.ID, &p.OrderID, &p.PaymentMethod, &p.AmountCents, &p.Status,
		&p.TransactionID, &p.WeChatPrepayData, &p.CallbackSignature,
		&p.CallbackReceivedAt, &p.RefundID, &p.RefundedAt,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load payment: %w", err)
	}
	return p, nil
}

func generateOrderNumber() string {
	return fmt.Sprintf("ORD-%s-%s", time.Now().Format("20060102"), randomHex(6))
}

func generateRefundID() string {
	return fmt.Sprintf("RF-%s-%s", time.Now().Format("20060102"), randomHex(6))
}

// randomHex returns n cryptographically random bytes encoded as uppercase hex.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return strings.ToUpper(hex.EncodeToString(b))
}
