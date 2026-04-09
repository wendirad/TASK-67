package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"

	"campusrec/internal/repository"
)

type PaymentService struct {
	orderRepo   *repository.OrderRepository
	auditRepo   *repository.AuditRepository
	merchantKey string
}

func NewPaymentService(orderRepo *repository.OrderRepository, auditRepo *repository.AuditRepository, merchantKey string) *PaymentService {
	return &PaymentService{
		orderRepo:   orderRepo,
		auditRepo:   auditRepo,
		merchantKey: merchantKey,
	}
}

// PaymentCallbackRequest represents the payment provider callback payload.
type PaymentCallbackRequest struct {
	TransactionID string `json:"transaction_id"`
	OrderNumber   string `json:"order_number"`
	AmountCents   int    `json:"amount_cents"`
	Status        string `json:"status"`
	NonceStr      string `json:"nonce_str"`
	Sign          string `json:"sign"`
}

// ProcessCallback verifies a payment callback signature, validates the payment,
// and atomically transitions the order to paid.
func (s *PaymentService) ProcessCallback(req *PaymentCallbackRequest) (int, string) {
	if req.TransactionID == "" || req.OrderNumber == "" || req.Sign == "" || req.NonceStr == "" {
		return 400, "Missing required callback fields"
	}
	if req.Status != "SUCCESS" {
		return 400, "Only SUCCESS callbacks are processed"
	}

	// Verify HMAC-SHA256 signature
	expectedSign := ComputeCallbackSignature(req.TransactionID, req.OrderNumber, req.AmountCents, req.Status, req.NonceStr, s.merchantKey)
	if !hmac.Equal([]byte(req.Sign), []byte(expectedSign)) {
		log.Printf("Payment callback signature mismatch for order %s: expected=%s got=%s", req.OrderNumber, expectedSign, req.Sign)
		return 403, "Invalid callback signature"
	}

	// Look up order
	order, err := s.orderRepo.FindOrderByNumber(req.OrderNumber)
	if err != nil {
		log.Printf("Error finding order %s: %v", req.OrderNumber, err)
		return 500, "Internal server error"
	}
	if order == nil {
		return 404, "Order not found"
	}

	// Verify amount matches
	if req.AmountCents != order.TotalCents {
		log.Printf("Payment amount mismatch for order %s: expected=%d got=%d", req.OrderNumber, order.TotalCents, req.AmountCents)
		return 422, "Payment amount does not match order total"
	}

	// Verify order is in correct state
	if order.Status != "pending_payment" {
		// Could be a duplicate callback — check idempotency
		if order.Status == "paid" {
			return 200, "Payment already confirmed"
		}
		return 422, "Order is not awaiting payment"
	}

	// Confirm payment atomically
	alreadyProcessed, err := s.orderRepo.ConfirmPayment(order.ID, req.TransactionID, req.Sign)
	if err != nil {
		log.Printf("Error confirming payment for order %s: %v", req.OrderNumber, err)
		return 500, "Internal server error"
	}
	if alreadyProcessed {
		return 200, "Payment already confirmed"
	}

	// Audit log
	oldVal := `{"status":"pending_payment"}`
	newVal := fmt.Sprintf(`{"status":"paid","transaction_id":"%s","amount_cents":%d}`, req.TransactionID, req.AmountCents)
	if err := s.auditRepo.Log("order", order.ID, "payment_confirmed", &oldVal, &newVal, "", "callback"); err != nil {
		log.Printf("Warning: failed to create audit log for payment %s: %v", req.TransactionID, err)
	}

	log.Printf("Payment confirmed: order=%s transaction=%s amount=%d", req.OrderNumber, req.TransactionID, req.AmountCents)
	return 200, ""
}

// SimulateCallback generates a valid callback payload for a given order and processes it.
// Used by staff/admin for testing and demo purposes.
func (s *PaymentService) SimulateCallback(orderID, performedBy string) (int, string) {
	order, err := s.orderRepo.FindByID(orderID)
	if err != nil {
		log.Printf("Error finding order %s: %v", orderID, err)
		return 500, "Internal server error"
	}
	if order == nil {
		return 404, "Order not found"
	}
	if order.Status != "pending_payment" {
		if order.Status == "paid" {
			return 200, "Order is already paid"
		}
		return 422, "Order is not awaiting payment"
	}

	// Generate valid callback data
	nonceStr := fmt.Sprintf("nonce_%s", order.ID[:8])
	transactionID := fmt.Sprintf("SIM-%s", order.OrderNumber)

	sign := ComputeCallbackSignature(transactionID, order.OrderNumber, order.TotalCents, "SUCCESS", nonceStr, s.merchantKey)

	req := &PaymentCallbackRequest{
		TransactionID: transactionID,
		OrderNumber:   order.OrderNumber,
		AmountCents:   order.TotalCents,
		Status:        "SUCCESS",
		NonceStr:      nonceStr,
		Sign:          sign,
	}

	code, msg := s.ProcessCallback(req)

	if code == 200 && msg == "" {
		log.Printf("Payment simulated: order=%s by=%s", orderID, performedBy)
	}

	return code, msg
}

// ComputeCallbackSignature computes HMAC-SHA256 over sorted key=value pairs.
func ComputeCallbackSignature(transactionID, orderNumber string, amountCents int, status, nonceStr, merchantKey string) string {
	params := map[string]string{
		"transaction_id": transactionID,
		"order_number":   orderNumber,
		"amount_cents":   fmt.Sprintf("%d", amountCents),
		"status":         status,
		"nonce_str":      nonceStr,
	}

	// Sort keys and build signing string
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, params[k]))
	}
	signingString := strings.Join(pairs, "&")

	mac := hmac.New(sha256.New, []byte(merchantKey))
	mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}
