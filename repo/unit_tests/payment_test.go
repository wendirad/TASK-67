package unit_tests

import (
	"testing"

	"campusrec/internal/services"
)

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
