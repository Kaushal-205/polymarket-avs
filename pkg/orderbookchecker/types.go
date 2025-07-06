package orderbookchecker

import (
	"math/big"
	"time"
)

// Order represents a single order in the orderbook
type Order struct {
	ID        string    `json:"id"`
	Side      string    `json:"side"`      // "buy" or "sell"
	Price     *big.Int  `json:"price"`     // Price in wei or smallest unit
	Quantity  *big.Int  `json:"quantity"`  // Quantity in wei or smallest unit
	Timestamp time.Time `json:"timestamp"` // When order was placed
	UserID    string    `json:"user_id"`   // User identifier
}

// Trade represents an executed trade from on-chain data
type Trade struct {
	ID           string   `json:"id"`
	BuyOrderID   string   `json:"buy_order_id"`
	SellOrderID  string   `json:"sell_order_id"`
	Price        *big.Int `json:"price"`
	Quantity     *big.Int `json:"quantity"`
	Timestamp    time.Time `json:"timestamp"`
	TxHash       string   `json:"tx_hash"`
	BlockNumber  uint64   `json:"block_number"`
}

// OrderbookSnapshot represents a snapshot of the orderbook at a specific point in time
type OrderbookSnapshot struct {
	SequenceNumber uint64   `json:"sequence_number"`
	Timestamp      time.Time `json:"timestamp"`
	MarketID       string   `json:"market_id"`
	Orders         []Order  `json:"orders"`
	MerkleRoot     string   `json:"merkle_root"`
	PrevHash       string   `json:"prev_hash"`
}

// VerificationResult represents the result of orderbook verification
type VerificationResult struct {
	Valid          bool     `json:"valid"`
	ErrorMessage   string   `json:"error_message,omitempty"`
	FailedTrades   []string `json:"failed_trades,omitempty"`
	VerifiedTrades int      `json:"verified_trades"`
	TotalTrades    int      `json:"total_trades"`
}

// OrderbookState represents the internal state of the orderbook during verification
type OrderbookState struct {
	BuyOrders  []Order // Sorted by price (highest first), then by timestamp
	SellOrders []Order // Sorted by price (lowest first), then by timestamp
} 