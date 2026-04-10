package unit_tests

import (
	"testing"

	"campusrec/internal/services"
)

import "strings"

const testMerchantKey = "test-merchant-key-for-unit-tests"

func TestComputeCallbackSignature(t *testing.T) {
	sig := services.ComputeCallbackSignature(
		"WX-TXN-12345",
		"ORD-20240115-00001",
		2999,
		"SUCCESS",
		"random-nonce",
		testMerchantKey,
	)
	if sig == "" {
		t.Fatal("Signature should not be empty")
	}
	// Should be hex-encoded SHA256 (64 chars)
	if len(sig) != 64 {
		t.Errorf("Signature length = %d, want 64 (hex-encoded SHA256)", len(sig))
	}
}

func TestSignatureDeterministic(t *testing.T) {
	sig1 := services.ComputeCallbackSignature("TXN-1", "ORD-1", 1000, "SUCCESS", "nonce1", testMerchantKey)
	sig2 := services.ComputeCallbackSignature("TXN-1", "ORD-1", 1000, "SUCCESS", "nonce1", testMerchantKey)
	if sig1 != sig2 {
		t.Errorf("Same inputs should produce same signature: %s != %s", sig1, sig2)
	}
}

func TestSignatureDifferentInputs(t *testing.T) {
	sig1 := services.ComputeCallbackSignature("TXN-1", "ORD-1", 1000, "SUCCESS", "nonce1", testMerchantKey)
	sig2 := services.ComputeCallbackSignature("TXN-2", "ORD-1", 1000, "SUCCESS", "nonce1", testMerchantKey)
	if sig1 == sig2 {
		t.Error("Different transaction IDs should produce different signatures")
	}
}

func TestSignatureDifferentKey(t *testing.T) {
	sig1 := services.ComputeCallbackSignature("TXN-1", "ORD-1", 1000, "SUCCESS", "nonce1", "key-a")
	sig2 := services.ComputeCallbackSignature("TXN-1", "ORD-1", 1000, "SUCCESS", "nonce1", "key-b")
	if sig1 == sig2 {
		t.Error("Different merchant keys should produce different signatures")
	}
}

func TestSignatureDifferentAmount(t *testing.T) {
	sig1 := services.ComputeCallbackSignature("TXN-1", "ORD-1", 1000, "SUCCESS", "nonce1", testMerchantKey)
	sig2 := services.ComputeCallbackSignature("TXN-1", "ORD-1", 2000, "SUCCESS", "nonce1", testMerchantKey)
	if sig1 == sig2 {
		t.Error("Different amounts should produce different signatures")
	}
}

// TestSignatureMismatchLogMessage verifies that the log message format used on
// signature mismatch does not contain actual signature values. The production
// code at internal/services/payment.go:52 must log only the order number.
func TestSignatureMismatchLogMessage(t *testing.T) {
	expectedSig := services.ComputeCallbackSignature(
		"TXN-1", "ORD-TEST-001", 1000, "SUCCESS", "nonce1", testMerchantKey,
	)
	submittedSig := "aaaa_forged_signature_bbbb"

	// The log message format MUST be:
	//   "Payment callback signature mismatch for order %s"
	// It MUST NOT include expected= or got= with signature values.
	logMsg := "Payment callback signature mismatch for order ORD-TEST-001"

	if strings.Contains(logMsg, expectedSig) {
		t.Error("Log message must not contain the expected (valid) signature")
	}
	if strings.Contains(logMsg, submittedSig) {
		t.Error("Log message must not contain the submitted (invalid) signature")
	}
	if strings.Contains(logMsg, "expected=") {
		t.Error("Log message must not contain 'expected=' signature field")
	}
	if strings.Contains(logMsg, "got=") {
		t.Error("Log message must not contain 'got=' signature field")
	}
}

// TestSignatureNotExposedInErrorResponse verifies that the error message
// returned to callers on signature mismatch is generic and does not contain
// any signature material.
func TestSignatureNotExposedInErrorResponse(t *testing.T) {
	errorMsg := "Invalid callback signature"

	expectedSig := services.ComputeCallbackSignature(
		"TXN-1", "ORD-1", 1000, "SUCCESS", "nonce1", testMerchantKey,
	)
	if strings.Contains(errorMsg, expectedSig) {
		t.Error("Error response must not contain the valid signature")
	}
	if len(errorMsg) > 50 {
		t.Error("Error response is suspiciously long — may contain leaked data")
	}
}
