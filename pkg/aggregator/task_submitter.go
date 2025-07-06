package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Layr-Labs/hourglass-avs-template/pkg/orderbookchecker"
	"github.com/Layr-Labs/hourglass-avs-template/pkg/publisher"
	"go.uber.org/zap"
)

// TaskSubmitter watches for new snapshots and submits verification tasks
type TaskSubmitter struct {
	logger       *zap.Logger
	snapshotDir  string
	publisher    *publisher.SnapshotPublisher
	lastSequence uint64
}

// TaskSubmissionResult represents the result of submitting a task
type TaskSubmissionResult struct {
	TaskID       string                              `json:"task_id"`
	SnapshotHash string                              `json:"snapshot_hash"`
	Snapshot     *orderbookchecker.OrderbookSnapshot `json:"snapshot"`
	Trades       []orderbookchecker.Trade            `json:"trades"`
	BatchID      string                              `json:"batch_id"`
	SubmittedAt  time.Time                           `json:"submitted_at"`
}

// NewTaskSubmitter creates a new task submitter
func NewTaskSubmitter(logger *zap.Logger, snapshotDir string) *TaskSubmitter {
	pub := publisher.NewSnapshotPublisher(logger, snapshotDir)

	return &TaskSubmitter{
		logger:       logger,
		snapshotDir:  snapshotDir,
		publisher:    pub,
		lastSequence: 0,
	}
}

// WatchAndSubmit watches for new snapshots and submits verification tasks
func (ts *TaskSubmitter) WatchAndSubmit(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ts.logger.Info("Starting snapshot watcher",
		zap.String("snapshot_dir", ts.snapshotDir),
		zap.Duration("interval", interval),
	)

	for {
		select {
		case <-ctx.Done():
			ts.logger.Info("Stopping snapshot watcher")
			return ctx.Err()
		case <-ticker.C:
			if err := ts.checkForNewSnapshots(); err != nil {
				ts.logger.Error("Failed to check for new snapshots", zap.Error(err))
			}
		}
	}
}

// checkForNewSnapshots scans for new snapshot files and submits tasks
func (ts *TaskSubmitter) checkForNewSnapshots() error {
	files, err := os.ReadDir(ts.snapshotDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist yet, nothing to do
			return nil
		}
		return fmt.Errorf("failed to read snapshot directory: %v", err)
	}

	var maxSequence uint64
	var newSnapshots []uint64

	// Find all snapshot files and determine the latest sequence number
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), "snapshot_") || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		// Extract sequence number from filename
		seqStr := strings.TrimPrefix(file.Name(), "snapshot_")
		seqStr = strings.TrimSuffix(seqStr, ".json")

		sequence, err := strconv.ParseUint(seqStr, 10, 64)
		if err != nil {
			ts.logger.Warn("Invalid snapshot filename", zap.String("filename", file.Name()))
			continue
		}

		if sequence > maxSequence {
			maxSequence = sequence
		}

		if sequence > ts.lastSequence {
			newSnapshots = append(newSnapshots, sequence)
		}
	}

	// Process new snapshots
	for _, sequence := range newSnapshots {
		if err := ts.processSnapshot(sequence); err != nil {
			ts.logger.Error("Failed to process snapshot",
				zap.Uint64("sequence", sequence),
				zap.Error(err),
			)
		}
	}

	ts.lastSequence = maxSequence
	return nil
}

// processSnapshot processes a single snapshot and submits a verification task
func (ts *TaskSubmitter) processSnapshot(sequence uint64) error {
	ts.logger.Info("Processing new snapshot", zap.Uint64("sequence", sequence))

	// Load snapshot
	snapshot, err := ts.publisher.LoadSnapshot(sequence)
	if err != nil {
		return fmt.Errorf("failed to load snapshot: %v", err)
	}

	// Load trades (may not exist for all snapshots)
	trades, err := ts.publisher.LoadTrades(sequence)
	if err != nil {
		// If trades file doesn't exist, continue with empty trades
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to load trades: %v", err)
		}
		trades = []orderbookchecker.Trade{}
	}

	// Create task input
	batchID := fmt.Sprintf("batch-%d", sequence)
	taskInput := ts.publisher.CreateTaskInput(snapshot, trades, batchID)

	// Submit task (in a real implementation, this would submit to the TaskMailbox)
	result, err := ts.submitVerificationTask(taskInput, snapshot, trades, batchID)
	if err != nil {
		return fmt.Errorf("failed to submit verification task: %v", err)
	}

	ts.logger.Info("Successfully submitted verification task",
		zap.String("task_id", result.TaskID),
		zap.Uint64("sequence", sequence),
		zap.String("batch_id", batchID),
		zap.Int("trades_count", len(trades)),
	)

	return nil
}

// submitVerificationTask submits a verification task (mock implementation)
func (ts *TaskSubmitter) submitVerificationTask(
	taskInput map[string]interface{},
	snapshot *orderbookchecker.OrderbookSnapshot,
	trades []orderbookchecker.Trade,
	batchID string,
) (*TaskSubmissionResult, error) {
	// In a real implementation, this would:
	// 1. Connect to the TaskMailbox contract
	// 2. Submit the task with the JSON payload
	// 3. Return the transaction hash as task ID

	// For now, we'll simulate this by saving the task to a file
	taskID := fmt.Sprintf("task-%d-%d", snapshot.SequenceNumber, time.Now().Unix())

	result := &TaskSubmissionResult{
		TaskID:       taskID,
		SnapshotHash: snapshot.MerkleRoot,
		Snapshot:     snapshot,
		Trades:       trades,
		BatchID:      batchID,
		SubmittedAt:  time.Now().UTC(),
	}

	// Save task submission record
	taskFile := filepath.Join(ts.snapshotDir, fmt.Sprintf("task_%s.json", taskID))
	taskData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task result: %v", err)
	}

	if err := os.WriteFile(taskFile, taskData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write task file: %v", err)
	}

	return result, nil
}

// GetTaskSubmissions returns all task submissions
func (ts *TaskSubmitter) GetTaskSubmissions() ([]*TaskSubmissionResult, error) {
	files, err := os.ReadDir(ts.snapshotDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot directory: %v", err)
	}

	var results []*TaskSubmissionResult

	for _, file := range files {
		if !strings.HasPrefix(file.Name(), "task_") || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		taskFile := filepath.Join(ts.snapshotDir, file.Name())
		data, err := os.ReadFile(taskFile)
		if err != nil {
			ts.logger.Warn("Failed to read task file", zap.String("file", file.Name()), zap.Error(err))
			continue
		}

		var result TaskSubmissionResult
		if err := json.Unmarshal(data, &result); err != nil {
			ts.logger.Warn("Failed to unmarshal task file", zap.String("file", file.Name()), zap.Error(err))
			continue
		}

		results = append(results, &result)
	}

	return results, nil
}

// SimulateTaskExecution simulates executing a verification task locally
func (ts *TaskSubmitter) SimulateTaskExecution(taskID string) (*orderbookchecker.VerificationResult, error) {
	// Load task submission
	taskFile := filepath.Join(ts.snapshotDir, fmt.Sprintf("task_%s.json", taskID))
	data, err := os.ReadFile(taskFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read task file: %v", err)
	}

	var submission TaskSubmissionResult
	if err := json.Unmarshal(data, &submission); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task submission: %v", err)
	}

	// Create verifier and run verification
	verifier := orderbookchecker.NewOrderbookVerifier(ts.logger)
	result, err := verifier.VerifySnapshot(submission.Trades, *submission.Snapshot)
	if err != nil {
		return nil, fmt.Errorf("verification failed: %v", err)
	}

	ts.logger.Info("Task execution simulation completed",
		zap.String("task_id", taskID),
		zap.Bool("valid", result.Valid),
		zap.Int("verified_trades", result.VerifiedTrades),
		zap.Int("failed_trades", len(result.FailedTrades)),
	)

	return result, nil
}
