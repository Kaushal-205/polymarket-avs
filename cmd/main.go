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
	SnapshotHash string                             `json:"snapshot_hash"`
	Snapshot     orderbookchecker.OrderbookSnapshot `json:"snapshot"`
	Trades       []orderbookchecker.Trade           `json:"trades"`
	TradeBatchID string                             `json:"trade_batch_id"`
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
	startTime := time.Now()

	tw.logger.Info("Starting task validation",
		zap.String("task_id", string(t.TaskId)),
		zap.Int("payload_size", len(t.Payload)),
		zap.Time("started_at", startTime),
	)

	// Parse task input
	var taskInput TaskInput
	if err := json.Unmarshal(t.Payload, &taskInput); err != nil {
		tw.logger.Error("Failed to parse task payload",
			zap.String("task_id", string(t.TaskId)),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)),
		)
		return fmt.Errorf("failed to parse task data: %v", err)
	}

	// Validate required fields
	if taskInput.SnapshotHash == "" {
		tw.logger.Error("Validation failed: missing snapshot_hash",
			zap.String("task_id", string(t.TaskId)),
			zap.Duration("duration", time.Since(startTime)),
		)
		return fmt.Errorf("snapshot_hash is required")
	}

	if taskInput.TradeBatchID == "" {
		tw.logger.Error("Validation failed: missing trade_batch_id",
			zap.String("task_id", string(t.TaskId)),
			zap.Duration("duration", time.Since(startTime)),
		)
		return fmt.Errorf("trade_batch_id is required")
	}

	if taskInput.Snapshot.SequenceNumber == 0 {
		tw.logger.Error("Validation failed: missing snapshot sequence_number",
			zap.String("task_id", string(t.TaskId)),
			zap.Duration("duration", time.Since(startTime)),
		)
		return fmt.Errorf("snapshot sequence_number is required")
	}

	if taskInput.Snapshot.MarketID == "" {
		tw.logger.Error("Validation failed: missing snapshot market_id",
			zap.String("task_id", string(t.TaskId)),
			zap.Duration("duration", time.Since(startTime)),
		)
		return fmt.Errorf("snapshot market_id is required")
	}

	if len(taskInput.Trades) == 0 {
		tw.logger.Error("Validation failed: empty trades array",
			zap.String("task_id", string(t.TaskId)),
			zap.Duration("duration", time.Since(startTime)),
		)
		return fmt.Errorf("trades array cannot be empty")
	}

	// Validate snapshot integrity (basic checks)
	if len(taskInput.Snapshot.Orders) == 0 {
		tw.logger.Error("Validation failed: empty orders in snapshot",
			zap.String("task_id", string(t.TaskId)),
			zap.Duration("duration", time.Since(startTime)),
		)
		return fmt.Errorf("snapshot must contain at least one order")
	}

	// Validate that all trades reference orders in the snapshot
	orderIDs := make(map[string]bool)
	for _, order := range taskInput.Snapshot.Orders {
		orderIDs[order.ID] = true
	}

	for _, trade := range taskInput.Trades {
		if !orderIDs[trade.BuyOrderID] {
			tw.logger.Error("Validation failed: trade references unknown buy order",
				zap.String("task_id", string(t.TaskId)),
				zap.String("trade_id", trade.ID),
				zap.String("buy_order_id", trade.BuyOrderID),
				zap.Duration("duration", time.Since(startTime)),
			)
			return fmt.Errorf("trade %s references unknown buy order %s", trade.ID, trade.BuyOrderID)
		}
		if !orderIDs[trade.SellOrderID] {
			tw.logger.Error("Validation failed: trade references unknown sell order",
				zap.String("task_id", string(t.TaskId)),
				zap.String("trade_id", trade.ID),
				zap.String("sell_order_id", trade.SellOrderID),
				zap.Duration("duration", time.Since(startTime)),
			)
			return fmt.Errorf("trade %s references unknown sell order %s", trade.ID, trade.SellOrderID)
		}
	}

	tw.logger.Info("Task validation completed successfully",
		zap.String("task_id", string(t.TaskId)),
		zap.String("snapshot_hash", taskInput.SnapshotHash),
		zap.String("trade_batch_id", taskInput.TradeBatchID),
		zap.String("market_id", taskInput.Snapshot.MarketID),
		zap.Uint64("sequence_number", taskInput.Snapshot.SequenceNumber),
		zap.Int("total_orders", len(taskInput.Snapshot.Orders)),
		zap.Int("total_trades", len(taskInput.Trades)),
		zap.Duration("validation_duration", time.Since(startTime)),
	)

	return nil
}

func (tw *TaskWorker) HandleTask(t *performerV1.TaskRequest) (*performerV1.TaskResponse, error) {
	startTime := time.Now()

	tw.logger.Info("Starting task execution",
		zap.String("task_id", string(t.TaskId)),
		zap.Int("payload_size", len(t.Payload)),
		zap.Time("started_at", startTime),
	)

	// Parse task input
	var taskInput TaskInput
	if err := json.Unmarshal(t.Payload, &taskInput); err != nil {
		tw.logger.Error("Failed to parse task payload during execution",
			zap.String("task_id", string(t.TaskId)),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)),
		)
		return nil, fmt.Errorf("failed to parse task data: %v", err)
	}

	tw.logger.Info("Task input parsed successfully",
		zap.String("task_id", string(t.TaskId)),
		zap.String("snapshot_hash", taskInput.SnapshotHash),
		zap.String("trade_batch_id", taskInput.TradeBatchID),
		zap.String("market_id", taskInput.Snapshot.MarketID),
		zap.Uint64("sequence_number", taskInput.Snapshot.SequenceNumber),
		zap.Int("orders_count", len(taskInput.Snapshot.Orders)),
		zap.Int("trades_count", len(taskInput.Trades)),
	)

	// Perform orderbook verification
	verificationStart := time.Now()
	result, err := tw.verifier.VerifySnapshot(taskInput.Trades, taskInput.Snapshot)
	verificationDuration := time.Since(verificationStart)

	if err != nil {
		tw.logger.Error("Orderbook verification failed",
			zap.String("task_id", string(t.TaskId)),
			zap.String("snapshot_hash", taskInput.SnapshotHash),
			zap.String("trade_batch_id", taskInput.TradeBatchID),
			zap.Error(err),
			zap.Duration("verification_duration", verificationDuration),
			zap.Duration("total_duration", time.Since(startTime)),
		)
		return nil, fmt.Errorf("verification failed: %v", err)
	}

	tw.logger.Info("Orderbook verification completed",
		zap.String("task_id", string(t.TaskId)),
		zap.Bool("valid", result.Valid),
		zap.Int("verified_trades", result.VerifiedTrades),
		zap.Int("total_trades", result.TotalTrades),
		zap.Int("failed_trades", len(result.FailedTrades)),
		zap.Duration("verification_duration", verificationDuration),
	)

	// Log detailed results for invalid settlements
	if !result.Valid {
		tw.logger.Warn("Settlement verification FAILED - potential fraud detected",
			zap.String("task_id", string(t.TaskId)),
			zap.String("snapshot_hash", taskInput.SnapshotHash),
			zap.String("trade_batch_id", taskInput.TradeBatchID),
			zap.String("market_id", taskInput.Snapshot.MarketID),
			zap.String("error_message", result.ErrorMessage),
			zap.Any("failed_trades", result.FailedTrades),
		)
	}

	// Prepare result
	resultData := map[string]interface{}{
		"verification_result": result,
		"snapshot_hash":       taskInput.SnapshotHash,
		"trade_batch_id":      taskInput.TradeBatchID,
		"verified_at":         time.Now().UTC(),
		"verifier_version":    "1.0.0",
		"performance_metrics": map[string]interface{}{
			"verification_duration_ms": verificationDuration.Milliseconds(),
			"total_duration_ms":        time.Since(startTime).Milliseconds(),
			"orders_processed":         len(taskInput.Snapshot.Orders),
			"trades_processed":         len(taskInput.Trades),
		},
	}

	resultBytes, err := json.Marshal(resultData)
	if err != nil {
		tw.logger.Error("Failed to marshal task result",
			zap.String("task_id", string(t.TaskId)),
			zap.Error(err),
			zap.Duration("total_duration", time.Since(startTime)),
		)
		return nil, fmt.Errorf("failed to marshal result: %v", err)
	}

	tw.logger.Info("Task execution completed successfully",
		zap.String("task_id", string(t.TaskId)),
		zap.String("snapshot_hash", taskInput.SnapshotHash),
		zap.String("trade_batch_id", taskInput.TradeBatchID),
		zap.Bool("settlement_valid", result.Valid),
		zap.Int("verified_trades", result.VerifiedTrades),
		zap.Int("total_trades", result.TotalTrades),
		zap.Int("failed_trades", len(result.FailedTrades)),
		zap.Int("result_size_bytes", len(resultBytes)),
		zap.Duration("verification_duration", verificationDuration),
		zap.Duration("total_duration", time.Since(startTime)),
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
