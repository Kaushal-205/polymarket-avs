package orderbookchecker

import (
	"math/big"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestOrderbookVerifier_VerifySnapshot_ValidTrade(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	verifier := NewOrderbookVerifier(logger)

	// Create a simple orderbook snapshot
	snapshot := OrderbookSnapshot{
		SequenceNumber: 1,
		Timestamp:      time.Now(),
		MarketID:       "BTC-USD",
		Orders: []Order{
			{
				ID:        "buy-1",
				Side:      "buy",
				Price:     big.NewInt(50200), // Higher than sell price to allow matching
				Quantity:  big.NewInt(1000),
				Timestamp: time.Now().Add(-2 * time.Minute),
				UserID:    "user1",
			},
			{
				ID:        "sell-1",
				Side:      "sell",
				Price:     big.NewInt(50100),
				Quantity:  big.NewInt(800),
				Timestamp: time.Now().Add(-1 * time.Minute),
				UserID:    "user2",
			},
		},
		MerkleRoot: "test-merkle-root",
		PrevHash:   "test-prev-hash",
	}

	// Create a valid trade
	trades := []Trade{
		{
			ID:          "trade-1",
			BuyOrderID:  "buy-1",
			SellOrderID: "sell-1",
			Price:       big.NewInt(50100), // Match sell price (price-time priority)
			Quantity:    big.NewInt(800),   // Match sell quantity
			Timestamp:   time.Now(),
			TxHash:      "0x123",
			BlockNumber: 1000,
		},
	}

	result, err := verifier.VerifySnapshot(trades, snapshot)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected valid result, got invalid: %s", result.ErrorMessage)
	}

	if result.VerifiedTrades != 1 {
		t.Errorf("Expected 1 verified trade, got %d", result.VerifiedTrades)
	}

	if len(result.FailedTrades) != 0 {
		t.Errorf("Expected 0 failed trades, got %d", len(result.FailedTrades))
	}
}

func TestOrderbookVerifier_VerifySnapshot_InvalidPrice(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	verifier := NewOrderbookVerifier(logger)

	// Create orderbook snapshot
	snapshot := OrderbookSnapshot{
		SequenceNumber: 1,
		Timestamp:      time.Now(),
		MarketID:       "BTC-USD",
		Orders: []Order{
			{
				ID:        "buy-1",
				Side:      "buy",
				Price:     big.NewInt(50200), // Higher than sell price to allow matching
				Quantity:  big.NewInt(1000),
				Timestamp: time.Now().Add(-2 * time.Minute),
				UserID:    "user1",
			},
			{
				ID:        "sell-1",
				Side:      "sell",
				Price:     big.NewInt(50100),
				Quantity:  big.NewInt(800),
				Timestamp: time.Now().Add(-1 * time.Minute),
				UserID:    "user2",
			},
		},
		MerkleRoot: "test-merkle-root",
		PrevHash:   "test-prev-hash",
	}

	// Create an invalid trade with price outside bid-ask spread
	trades := []Trade{
		{
			ID:          "trade-1",
			BuyOrderID:  "buy-1",
			SellOrderID: "sell-1",
			Price:       big.NewInt(49000), // Invalid: below buy order price
			Quantity:    big.NewInt(800),
			Timestamp:   time.Now(),
			TxHash:      "0x123",
			BlockNumber: 1000,
		},
	}

	result, err := verifier.VerifySnapshot(trades, snapshot)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid result due to price mismatch")
	}

	if len(result.FailedTrades) != 1 {
		t.Errorf("Expected 1 failed trade, got %d", len(result.FailedTrades))
	}
}

func TestOrderbookVerifier_VerifySnapshot_ExcessiveQuantity(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	verifier := NewOrderbookVerifier(logger)

	// Create orderbook snapshot
	snapshot := OrderbookSnapshot{
		SequenceNumber: 1,
		Timestamp:      time.Now(),
		MarketID:       "BTC-USD",
		Orders: []Order{
			{
				ID:        "buy-1",
				Side:      "buy",
				Price:     big.NewInt(50200), // Higher than sell price to allow matching
				Quantity:  big.NewInt(500), // Small quantity
				Timestamp: time.Now().Add(-2 * time.Minute),
				UserID:    "user1",
			},
			{
				ID:        "sell-1",
				Side:      "sell",
				Price:     big.NewInt(50100),
				Quantity:  big.NewInt(800),
				Timestamp: time.Now().Add(-1 * time.Minute),
				UserID:    "user2",
			},
		},
		MerkleRoot: "test-merkle-root",
		PrevHash:   "test-prev-hash",
	}

	// Create trade with quantity exceeding buy order
	trades := []Trade{
		{
			ID:          "trade-1",
			BuyOrderID:  "buy-1",
			SellOrderID: "sell-1",
			Price:       big.NewInt(50100),
			Quantity:    big.NewInt(600), // Exceeds buy order quantity of 500
			Timestamp:   time.Now(),
			TxHash:      "0x123",
			BlockNumber: 1000,
		},
	}

	result, err := verifier.VerifySnapshot(trades, snapshot)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid result due to quantity constraint violation")
	}

	if len(result.FailedTrades) != 1 {
		t.Errorf("Expected 1 failed trade, got %d", len(result.FailedTrades))
	}
}

func TestOrderbookVerifier_VerifySnapshot_MissingOrder(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	verifier := NewOrderbookVerifier(logger)

	// Create orderbook snapshot with only one order
	snapshot := OrderbookSnapshot{
		SequenceNumber: 1,
		Timestamp:      time.Now(),
		MarketID:       "BTC-USD",
		Orders: []Order{
			{
				ID:        "buy-1",
				Side:      "buy",
				Price:     big.NewInt(50000),
				Quantity:  big.NewInt(1000),
				Timestamp: time.Now().Add(-2 * time.Minute),
				UserID:    "user1",
			},
		},
		MerkleRoot: "test-merkle-root",
		PrevHash:   "test-prev-hash",
	}

	// Create trade referencing non-existent sell order
	trades := []Trade{
		{
			ID:          "trade-1",
			BuyOrderID:  "buy-1",
			SellOrderID: "sell-nonexistent",
			Price:       big.NewInt(50100),
			Quantity:    big.NewInt(800),
			Timestamp:   time.Now(),
			TxHash:      "0x123",
			BlockNumber: 1000,
		},
	}

	result, err := verifier.VerifySnapshot(trades, snapshot)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid result due to missing sell order")
	}

	if len(result.FailedTrades) != 1 {
		t.Errorf("Expected 1 failed trade, got %d", len(result.FailedTrades))
	}
}

func TestOrderbookVerifier_BuildOrderbookState(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	verifier := NewOrderbookVerifier(logger)

	orders := []Order{
		{
			ID:        "buy-high",
			Side:      "buy",
			Price:     big.NewInt(52000),
			Quantity:  big.NewInt(1000),
			Timestamp: time.Now().Add(-3 * time.Minute),
			UserID:    "user1",
		},
		{
			ID:        "buy-low",
			Side:      "buy",
			Price:     big.NewInt(50000),
			Quantity:  big.NewInt(1000),
			Timestamp: time.Now().Add(-2 * time.Minute),
			UserID:    "user2",
		},
		{
			ID:        "sell-low",
			Side:      "sell",
			Price:     big.NewInt(50100),
			Quantity:  big.NewInt(800),
			Timestamp: time.Now().Add(-1 * time.Minute),
			UserID:    "user3",
		},
		{
			ID:        "sell-high",
			Side:      "sell",
			Price:     big.NewInt(52100),
			Quantity:  big.NewInt(800),
			Timestamp: time.Now(),
			UserID:    "user4",
		},
	}

	state, err := verifier.buildOrderbookState(orders)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check buy orders are sorted by price (highest first)
	if len(state.BuyOrders) != 2 {
		t.Errorf("Expected 2 buy orders, got %d", len(state.BuyOrders))
	}

	if state.BuyOrders[0].ID != "buy-high" {
		t.Errorf("Expected first buy order to be 'buy-high', got %s", state.BuyOrders[0].ID)
	}

	if state.BuyOrders[1].ID != "buy-low" {
		t.Errorf("Expected second buy order to be 'buy-low', got %s", state.BuyOrders[1].ID)
	}

	// Check sell orders are sorted by price (lowest first)
	if len(state.SellOrders) != 2 {
		t.Errorf("Expected 2 sell orders, got %d", len(state.SellOrders))
	}

	if state.SellOrders[0].ID != "sell-low" {
		t.Errorf("Expected first sell order to be 'sell-low', got %s", state.SellOrders[0].ID)
	}

	if state.SellOrders[1].ID != "sell-high" {
		t.Errorf("Expected second sell order to be 'sell-high', got %s", state.SellOrders[1].ID)
	}
}

func TestOrderbookVerifier_TimePriority(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	verifier := NewOrderbookVerifier(logger)

	baseTime := time.Now().Add(-10 * time.Minute)

	// Create orderbook with multiple orders at same price
	snapshot := OrderbookSnapshot{
		SequenceNumber: 1,
		Timestamp:      time.Now(),
		MarketID:       "BTC-USD",
		Orders: []Order{
			{
				ID:        "buy-early",
				Side:      "buy",
				Price:     big.NewInt(50200), // Higher than sell price to allow matching
				Quantity:  big.NewInt(1000),
				Timestamp: baseTime, // Earlier timestamp
				UserID:    "user1",
			},
			{
				ID:        "buy-late",
				Side:      "buy",
				Price:     big.NewInt(50200), // Same price
				Quantity:  big.NewInt(1000),
				Timestamp: baseTime.Add(1 * time.Minute), // Later timestamp
				UserID:    "user2",
			},
			{
				ID:        "sell-1",
				Side:      "sell",
				Price:     big.NewInt(50100),
				Quantity:  big.NewInt(800),
				Timestamp: baseTime.Add(2 * time.Minute),
				UserID:    "user3",
			},
		},
		MerkleRoot: "test-merkle-root",
		PrevHash:   "test-prev-hash",
	}

	// Trade should match the earlier order (buy-early) due to time priority
	validTrades := []Trade{
		{
			ID:          "trade-1",
			BuyOrderID:  "buy-early", // Earlier order should be matched first
			SellOrderID: "sell-1",
			Price:       big.NewInt(50100),
			Quantity:    big.NewInt(800),
			Timestamp:   time.Now(),
			TxHash:      "0x123",
			BlockNumber: 1000,
		},
	}

	result, err := verifier.VerifySnapshot(validTrades, snapshot)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected valid result for time priority respected, got invalid: %s", result.ErrorMessage)
	}
} 