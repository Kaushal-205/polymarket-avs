package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Layr-Labs/hourglass-avs-template/pkg/aggregator"
	"github.com/Layr-Labs/hourglass-avs-template/pkg/publisher"
	"go.uber.org/zap"
)

func main() {
	var (
		snapshotDir = flag.String("snapshot-dir", "./demo-snapshots", "Directory for snapshots")
		marketID    = flag.String("market", "TRUMP-2024-WIN", "Market ID")
		mode        = flag.String("mode", "full", "Demo mode: full, publish, watch, verify")
		taskID      = flag.String("task-id", "", "Task ID for verification (verify mode)")
		interval    = flag.Duration("interval", 5*time.Second, "Watch interval")
	)
	flag.Parse()

	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	switch *mode {
	case "full":
		runFullDemo(logger, *snapshotDir, *marketID, *interval)
	case "publish":
		runPublishDemo(logger, *snapshotDir, *marketID)
	case "watch":
		runWatchDemo(logger, *snapshotDir, *interval)
	case "verify":
		runVerifyDemo(logger, *snapshotDir, *taskID)
	default:
		fmt.Printf("Invalid mode: %s\n", *mode)
		fmt.Printf("Available modes: full, publish, watch, verify\n")
		os.Exit(1)
	}
}

// runFullDemo runs the complete end-to-end demo
func runFullDemo(logger *zap.Logger, snapshotDir, marketID string, interval time.Duration) {
	fmt.Println("🚀 Starting Polymarket AVS Full Demo")
	fmt.Println("===================================")

	// Step 1: Publish a snapshot with sample data
	fmt.Println("\n📸 Step 1: Publishing orderbook snapshot...")
	pub := publisher.NewSnapshotPublisher(logger, snapshotDir)
	orders, trades := pub.GenerateSampleData(marketID)

	snapshot, err := pub.PublishSnapshot(marketID, orders, trades)
	if err != nil {
		logger.Fatal("Failed to publish snapshot", zap.Error(err))
	}

	fmt.Printf("✅ Snapshot published:\n")
	fmt.Printf("   - Sequence: %d\n", snapshot.SequenceNumber)
	fmt.Printf("   - Market: %s\n", snapshot.MarketID)
	fmt.Printf("   - Orders: %d\n", len(orders))
	fmt.Printf("   - Trades: %d\n", len(trades))
	fmt.Printf("   - Merkle Root: %s\n", snapshot.MerkleRoot)

	// Step 2: Start task submitter to watch for snapshots
	fmt.Println("\n👁️  Step 2: Starting task submitter (watching for snapshots)...")
	submitter := aggregator.NewTaskSubmitter(logger, snapshotDir)

	// Manually trigger processing of the snapshot we just created
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		if err := submitter.WatchAndSubmit(ctx, 500*time.Millisecond); err != nil && err != context.DeadlineExceeded {
			logger.Error("Watcher failed", zap.Error(err))
		}
	}()

	// Wait for task submission
	time.Sleep(3 * time.Second)
	cancel() // Stop the watcher

	// Step 3: Check submitted tasks
	fmt.Println("\n📋 Step 3: Checking submitted tasks...")
	tasks, err := submitter.GetTaskSubmissions()
	if err != nil {
		logger.Fatal("Failed to get task submissions", zap.Error(err))
	}

	if len(tasks) == 0 {
		fmt.Println("❌ No tasks were submitted")
		return
	}

	task := tasks[len(tasks)-1] // Get the latest task
	fmt.Printf("✅ Task submitted:\n")
	fmt.Printf("   - Task ID: %s\n", task.TaskID)
	fmt.Printf("   - Batch ID: %s\n", task.BatchID)
	fmt.Printf("   - Snapshot Hash: %s\n", task.SnapshotHash)
	fmt.Printf("   - Trades: %d\n", len(task.Trades))

	// Step 4: Simulate task execution (verification)
	fmt.Println("\n🔍 Step 4: Simulating AVS verification...")
	result, err := submitter.SimulateTaskExecution(task.TaskID)
	if err != nil {
		logger.Fatal("Failed to simulate task execution", zap.Error(err))
	}

	fmt.Printf("✅ Verification completed:\n")
	fmt.Printf("   - Valid: %t\n", result.Valid)
	fmt.Printf("   - Verified Trades: %d/%d\n", result.VerifiedTrades, result.TotalTrades)
	if len(result.FailedTrades) > 0 {
		fmt.Printf("   - Failed Trades: %v\n", result.FailedTrades)
		fmt.Printf("   - Error: %s\n", result.ErrorMessage)
	}

	// Step 5: Show what would happen next
	fmt.Println("\n🏁 Step 5: Next steps in production:")
	if result.Valid {
		fmt.Println("   ✅ Settlement is valid - no action needed")
		fmt.Println("   📝 Result would be signed and submitted to aggregator")
	} else {
		fmt.Println("   ❌ Settlement is INVALID - challenge would be submitted")
		fmt.Println("   ⚖️  Challenge contract would freeze the settlement")
		fmt.Println("   💰 Fraudulent operator would be slashed")
	}

	fmt.Println("\n🎉 Demo completed successfully!")
	fmt.Printf("📁 Demo files saved in: %s\n", snapshotDir)
}

// runPublishDemo publishes sample snapshots
func runPublishDemo(logger *zap.Logger, snapshotDir, marketID string) {
	fmt.Println("📸 Publishing snapshot demo...")

	pub := publisher.NewSnapshotPublisher(logger, snapshotDir)
	orders, trades := pub.GenerateSampleData(marketID)

	snapshot, err := pub.PublishSnapshot(marketID, orders, trades)
	if err != nil {
		logger.Fatal("Failed to publish snapshot", zap.Error(err))
	}

	fmt.Printf("Snapshot published: %s\n", snapshot.MerkleRoot)

	// Also create a task input file
	taskInput := pub.CreateTaskInput(snapshot, trades, fmt.Sprintf("batch-%d", snapshot.SequenceNumber))
	taskData, _ := json.MarshalIndent(taskInput, "", "  ")

	taskFile := fmt.Sprintf("%s/task_input_%d.json", snapshotDir, snapshot.SequenceNumber)
	os.WriteFile(taskFile, taskData, 0644)
	fmt.Printf("Task input saved: %s\n", taskFile)
}

// runWatchDemo runs the snapshot watcher
func runWatchDemo(logger *zap.Logger, snapshotDir string, interval time.Duration) {
	fmt.Printf("👁️  Starting snapshot watcher (interval: %v)...\n", interval)
	fmt.Println("Press Ctrl+C to stop")

	submitter := aggregator.NewTaskSubmitter(logger, snapshotDir)

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, stopping...")
		cancel()
	}()

	if err := submitter.WatchAndSubmit(ctx, interval); err != nil && err != context.Canceled {
		logger.Fatal("Watcher failed", zap.Error(err))
	}

	fmt.Println("Watcher stopped")
}

// runVerifyDemo verifies a specific task
func runVerifyDemo(logger *zap.Logger, snapshotDir, taskID string) {
	if taskID == "" {
		fmt.Println("Please specify a task ID with -task-id")
		os.Exit(1)
	}

	fmt.Printf("🔍 Verifying task: %s\n", taskID)

	submitter := aggregator.NewTaskSubmitter(logger, snapshotDir)
	result, err := submitter.SimulateTaskExecution(taskID)
	if err != nil {
		logger.Fatal("Verification failed", zap.Error(err))
	}

	fmt.Printf("Verification result:\n")
	fmt.Printf("  Valid: %t\n", result.Valid)
	fmt.Printf("  Verified: %d/%d trades\n", result.VerifiedTrades, result.TotalTrades)
	if !result.Valid {
		fmt.Printf("  Error: %s\n", result.ErrorMessage)
		fmt.Printf("  Failed trades: %v\n", result.FailedTrades)
	}
}
