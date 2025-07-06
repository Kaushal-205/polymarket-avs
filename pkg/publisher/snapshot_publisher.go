package publisher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Layr-Labs/hourglass-avs-template/pkg/orderbookchecker"
	"go.uber.org/zap"
)

// SnapshotPublisher handles the creation and publishing of orderbook snapshots
type SnapshotPublisher struct {
	logger      *zap.Logger
	outputDir   string
	sequenceNum uint64
	prevHash    string
}

// NewSnapshotPublisher creates a new snapshot publisher
func NewSnapshotPublisher(logger *zap.Logger, outputDir string) *SnapshotPublisher {
	return &SnapshotPublisher{
		logger:      logger,
		outputDir:   outputDir,
		sequenceNum: 1,
		prevHash:    "0x0000000000000000000000000000000000000000000000000000000000000000",
	}
}

// PublishSnapshot creates and publishes a new orderbook snapshot
func (sp *SnapshotPublisher) PublishSnapshot(marketID string, orders []orderbookchecker.Order, trades []orderbookchecker.Trade) (*orderbookchecker.OrderbookSnapshot, error) {
	timestamp := time.Now().UTC()

	// Calculate merkle root for orders
	merkleRoot, err := sp.calculateMerkleRoot(orders)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate merkle root: %v", err)
	}

	// Create snapshot
	snapshot := &orderbookchecker.OrderbookSnapshot{
		SequenceNumber: sp.sequenceNum,
		Timestamp:      timestamp,
		MarketID:       marketID,
		Orders:         orders,
		MerkleRoot:     merkleRoot,
		PrevHash:       sp.prevHash,
	}

	// Save snapshot to disk
	if err := sp.saveSnapshot(snapshot); err != nil {
		return nil, fmt.Errorf("failed to save snapshot: %v", err)
	}

	// Save trades separately
	if len(trades) > 0 {
		if err := sp.saveTrades(trades, sp.sequenceNum); err != nil {
			return nil, fmt.Errorf("failed to save trades: %v", err)
		}
	}

	// Update state for next snapshot
	sp.prevHash = sp.hashSnapshot(snapshot)
	sp.sequenceNum++

	sp.logger.Sugar().Infow("Published snapshot",
		"sequence_number", snapshot.SequenceNumber,
		"market_id", marketID,
		"orders_count", len(orders),
		"trades_count", len(trades),
		"merkle_root", merkleRoot,
	)

	return snapshot, nil
}

// calculateMerkleRoot computes a simple merkle root for the orders
func (sp *SnapshotPublisher) calculateMerkleRoot(orders []orderbookchecker.Order) (string, error) {
	if len(orders) == 0 {
		return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
	}

	// Sort orders by ID for deterministic hashing
	sortedOrders := make([]orderbookchecker.Order, len(orders))
	copy(sortedOrders, orders)
	sort.Slice(sortedOrders, func(i, j int) bool {
		return sortedOrders[i].ID < sortedOrders[j].ID
	})

	// Create leaf hashes
	var leaves []string
	for _, order := range sortedOrders {
		orderBytes, err := json.Marshal(order)
		if err != nil {
			return "", fmt.Errorf("failed to marshal order %s: %v", order.ID, err)
		}
		hash := sha256.Sum256(orderBytes)
		leaves = append(leaves, hex.EncodeToString(hash[:]))
	}

	// Build merkle tree (simplified - just hash all leaves together for now)
	// In production, you'd want a proper merkle tree implementation
	allLeaves := ""
	for _, leaf := range leaves {
		allLeaves += leaf
	}

	finalHash := sha256.Sum256([]byte(allLeaves))
	return "0x" + hex.EncodeToString(finalHash[:]), nil
}

// hashSnapshot creates a hash of the snapshot for the next prevHash
func (sp *SnapshotPublisher) hashSnapshot(snapshot *orderbookchecker.OrderbookSnapshot) string {
	data := fmt.Sprintf("%d-%s-%s-%s",
		snapshot.SequenceNumber,
		snapshot.MarketID,
		snapshot.MerkleRoot,
		snapshot.Timestamp.Format(time.RFC3339),
	)
	hash := sha256.Sum256([]byte(data))
	return "0x" + hex.EncodeToString(hash[:])
}

// saveSnapshot saves the snapshot to disk
func (sp *SnapshotPublisher) saveSnapshot(snapshot *orderbookchecker.OrderbookSnapshot) error {
	// Ensure output directory exists
	if err := os.MkdirAll(sp.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Save snapshot
	filename := filepath.Join(sp.outputDir, fmt.Sprintf("snapshot_%d.json", snapshot.SequenceNumber))
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot file: %v", err)
	}

	return nil
}

// saveTrades saves the trades to disk
func (sp *SnapshotPublisher) saveTrades(trades []orderbookchecker.Trade, sequenceNum uint64) error {
	filename := filepath.Join(sp.outputDir, fmt.Sprintf("trades_%d.json", sequenceNum))
	data, err := json.MarshalIndent(trades, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trades: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write trades file: %v", err)
	}

	return nil
}

// LoadSnapshot loads a snapshot from disk
func (sp *SnapshotPublisher) LoadSnapshot(sequenceNum uint64) (*orderbookchecker.OrderbookSnapshot, error) {
	filename := filepath.Join(sp.outputDir, fmt.Sprintf("snapshot_%d.json", sequenceNum))
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %v", err)
	}

	var snapshot orderbookchecker.OrderbookSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %v", err)
	}

	return &snapshot, nil
}

// LoadTrades loads trades from disk
func (sp *SnapshotPublisher) LoadTrades(sequenceNum uint64) ([]orderbookchecker.Trade, error) {
	filename := filepath.Join(sp.outputDir, fmt.Sprintf("trades_%d.json", sequenceNum))
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read trades file: %v", err)
	}

	var trades []orderbookchecker.Trade
	if err := json.Unmarshal(data, &trades); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trades: %v", err)
	}

	return trades, nil
}

// GenerateSampleData creates sample orderbook data for testing
func (sp *SnapshotPublisher) GenerateSampleData(marketID string) ([]orderbookchecker.Order, []orderbookchecker.Trade) {
	baseTime := time.Now().UTC().Add(-5 * time.Minute)

	orders := []orderbookchecker.Order{
		{
			ID:        "order-buy-001",
			Side:      "buy",
			Price:     big.NewInt(530000000000000000),  // 0.53 ETH
			Quantity:  big.NewInt(1000000000000000000), // 1 ETH
			Timestamp: baseTime,
			UserID:    "user-alice",
		},
		{
			ID:        "order-buy-002",
			Side:      "buy",
			Price:     big.NewInt(515000000000000000),  // 0.515 ETH
			Quantity:  big.NewInt(2000000000000000000), // 2 ETH
			Timestamp: baseTime.Add(1 * time.Minute),
			UserID:    "user-bob",
		},
		{
			ID:        "order-sell-001",
			Side:      "sell",
			Price:     big.NewInt(530000000000000000),  // 0.53 ETH
			Quantity:  big.NewInt(1500000000000000000), // 1.5 ETH
			Timestamp: baseTime.Add(30 * time.Second),
			UserID:    "user-charlie",
		},
		{
			ID:        "order-sell-002",
			Side:      "sell",
			Price:     big.NewInt(525000000000000000), // 0.525 ETH
			Quantity:  big.NewInt(800000000000000000), // 0.8 ETH
			Timestamp: baseTime.Add(90 * time.Second),
			UserID:    "user-diana",
		},
	}

	trades := []orderbookchecker.Trade{
		{
			ID:          "trade-001",
			BuyOrderID:  "order-buy-001",
			SellOrderID: "order-sell-002",
			Price:       big.NewInt(525000000000000000), // 0.525 ETH (matches sell order price)
			Quantity:    big.NewInt(800000000000000000), // 0.8 ETH
			Timestamp:   baseTime.Add(2 * time.Minute),
			TxHash:      "0xdef123456789abcdef123456789abcdef12345678",
			BlockNumber: 12345678,
		},
	}

	return orders, trades
}

// CreateTaskInput creates a TaskInput from snapshot and trades for AVS processing
func (sp *SnapshotPublisher) CreateTaskInput(snapshot *orderbookchecker.OrderbookSnapshot, trades []orderbookchecker.Trade, tradeBatchID string) map[string]interface{} {
	return map[string]interface{}{
		"snapshot_hash":  snapshot.MerkleRoot,
		"trade_batch_id": tradeBatchID,
		"snapshot":       snapshot,
		"trades":         trades,
	}
}
