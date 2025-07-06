package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Layr-Labs/hourglass-avs-template/pkg/publisher"
	"go.uber.org/zap"
)

func main() {
	var (
		outputDir = flag.String("output", "./snapshots", "Output directory for snapshots")
		marketID  = flag.String("market", "TRUMP-2024-WIN", "Market ID for the snapshot")
		generate  = flag.Bool("generate", false, "Generate sample data")
		taskFile  = flag.String("task-file", "", "Output file for task input JSON")
	)
	flag.Parse()

	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	pub := publisher.NewSnapshotPublisher(logger, *outputDir)

	if *generate {
		// Generate sample data
		orders, trades := pub.GenerateSampleData(*marketID)

		// Publish snapshot
		snapshot, err := pub.PublishSnapshot(*marketID, orders, trades)
		if err != nil {
			logger.Fatal("Failed to publish snapshot", zap.Error(err))
		}

		logger.Info("Successfully published snapshot",
			zap.Uint64("sequence_number", snapshot.SequenceNumber),
			zap.String("market_id", *marketID),
			zap.String("merkle_root", snapshot.MerkleRoot),
		)

		// Generate task input if requested
		if *taskFile != "" {
			taskInput := pub.CreateTaskInput(snapshot, trades, fmt.Sprintf("batch-%d", snapshot.SequenceNumber))

			taskData, err := json.MarshalIndent(taskInput, "", "  ")
			if err != nil {
				logger.Fatal("Failed to marshal task input", zap.Error(err))
			}

			if err := os.WriteFile(*taskFile, taskData, 0644); err != nil {
				logger.Fatal("Failed to write task file", zap.Error(err))
			}

			logger.Info("Task input written to file", zap.String("file", *taskFile))
		}

		fmt.Printf("Snapshot published successfully!\n")
		fmt.Printf("Sequence Number: %d\n", snapshot.SequenceNumber)
		fmt.Printf("Market ID: %s\n", snapshot.MarketID)
		fmt.Printf("Merkle Root: %s\n", snapshot.MerkleRoot)
		fmt.Printf("Orders: %d\n", len(snapshot.Orders))
		fmt.Printf("Trades: %d\n", len(trades))
	} else {
		fmt.Printf("Usage: %s -generate [options]\n", os.Args[0])
		flag.PrintDefaults()
	}
}
