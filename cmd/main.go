package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Layr-Labs/hourglass-avs-template/pkg/orderbookchecker"
	"github.com/Layr-Labs/hourglass-monorepo/ponos/pkg/performer/server"
	performerV1 "github.com/Layr-Labs/protocol-apis/gen/protos/eigenlayer/hourglass/v1/performer"
	"go.uber.org/zap"
)

// This offchain binary is run by Operators running the Hourglass Executor. It contains
// the business logic of the AVS and performs worked based on the tasked sent to it.
// The Hourglass Aggregator ingests tasks from the TaskMailbox and distributes work
// to Executors configured to run the AVS Performer. Performers execute the work and
// return the result to the Executor where the result is signed and return to the
// Aggregator to place in the outbox once the signing threshold is met.

// TaskInput represents the input data for orderbook verification tasks
type TaskInput struct {
	SnapshotHash string                            `json:"snapshot_hash"`
	Snapshot     orderbookchecker.OrderbookSnapshot `json:"snapshot"`
	Trades       []orderbookchecker.Trade           `json:"trades"`
	TradeBatchID string                            `json:"trade_batch_id"`
}

type TaskWorker struct {
	logger   *zap.Logger
	verifier *orderbookchecker.OrderbookVerifier
}

func NewTaskWorker(logger *zap.Logger) *TaskWorker {
	return &TaskWorker{
		logger:   logger,
		verifier: orderbookchecker.NewOrderbookVerifier(logger),
	}
}

func (tw *TaskWorker) ValidateTask(t *performerV1.TaskRequest) error {
	tw.logger.Sugar().Infow("Validating task",
		zap.Any("task", t),
	)

	// Parse task input
	var taskInput TaskInput
	if err := json.Unmarshal(t.Payload, &taskInput); err != nil {
		return fmt.Errorf("failed to parse task data: %v", err)
	}

	// Validate required fields
	if taskInput.SnapshotHash == "" {
		return fmt.Errorf("snapshot_hash is required")
	}

	if taskInput.TradeBatchID == "" {
		return fmt.Errorf("trade_batch_id is required")
	}

	if taskInput.Snapshot.SequenceNumber == 0 {
		return fmt.Errorf("snapshot sequence_number is required")
	}

	if taskInput.Snapshot.MarketID == "" {
		return fmt.Errorf("snapshot market_id is required")
	}

	if len(taskInput.Trades) == 0 {
		return fmt.Errorf("trades array cannot be empty")
	}

	// Validate snapshot integrity (basic checks)
	if len(taskInput.Snapshot.Orders) == 0 {
		return fmt.Errorf("snapshot must contain at least one order")
	}

	// Validate that all trades reference orders in the snapshot
	orderIDs := make(map[string]bool)
	for _, order := range taskInput.Snapshot.Orders {
		orderIDs[order.ID] = true
	}

	for _, trade := range taskInput.Trades {
		if !orderIDs[trade.BuyOrderID] {
			return fmt.Errorf("trade %s references unknown buy order %s", trade.ID, trade.BuyOrderID)
		}
		if !orderIDs[trade.SellOrderID] {
			return fmt.Errorf("trade %s references unknown sell order %s", trade.ID, trade.SellOrderID)
		}
	}

	tw.logger.Sugar().Infow("Task validation passed",
		"snapshot_hash", taskInput.SnapshotHash,
		"trade_batch_id", taskInput.TradeBatchID,
		"total_orders", len(taskInput.Snapshot.Orders),
		"total_trades", len(taskInput.Trades),
	)

	return nil
}

func (tw *TaskWorker) HandleTask(t *performerV1.TaskRequest) (*performerV1.TaskResponse, error) {
	tw.logger.Sugar().Infow("Handling task",
		zap.Any("task", t),
	)

	// Parse task input
	var taskInput TaskInput
	if err := json.Unmarshal(t.Payload, &taskInput); err != nil {
		return nil, fmt.Errorf("failed to parse task data: %v", err)
	}

	// Perform orderbook verification
	result, err := tw.verifier.VerifySnapshot(taskInput.Trades, taskInput.Snapshot)
	if err != nil {
		tw.logger.Sugar().Errorw("Verification failed",
			"error", err,
			"snapshot_hash", taskInput.SnapshotHash,
			"trade_batch_id", taskInput.TradeBatchID,
		)
		return nil, fmt.Errorf("verification failed: %v", err)
	}

	// Prepare result
	resultData := map[string]interface{}{
		"verification_result": result,
		"snapshot_hash":       taskInput.SnapshotHash,
		"trade_batch_id":      taskInput.TradeBatchID,
		"verified_at":         time.Now().UTC(),
		"verifier_version":    "1.0.0",
	}

	resultBytes, err := json.Marshal(resultData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %v", err)
	}

	tw.logger.Sugar().Infow("Task completed successfully",
		"valid", result.Valid,
		"verified_trades", result.VerifiedTrades,
		"total_trades", result.TotalTrades,
		"failed_trades", len(result.FailedTrades),
	)

	return &performerV1.TaskResponse{
		TaskId: t.TaskId,
		Result: resultBytes,
	}, nil
}

func main() {
	ctx := context.Background()
	l, _ := zap.NewProduction()

	w := NewTaskWorker(l)

	pp, err := server.NewPonosPerformerWithRpcServer(&server.PonosPerformerConfig{
		Port:    8080,
		Timeout: 5 * time.Second,
	}, w, l)
	if err != nil {
		panic(fmt.Errorf("failed to create performer: %w", err))
	}

	if err := pp.Start(ctx); err != nil {
		panic(err)
	}
}
