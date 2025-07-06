package main

import (
	"encoding/json"
	"github.com/Layr-Labs/hourglass-avs-template/pkg/orderbookchecker"
	performerV1 "github.com/Layr-Labs/protocol-apis/gen/protos/eigenlayer/hourglass/v1/performer"
	"go.uber.org/zap"
	"math/big"
	"testing"
)

func Test_TaskRequestPayload(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Errorf("Failed to create logger: %v", err)
	}

	taskWorker := NewTaskWorker(logger)

	// Create valid JSON payload
	taskInput := TaskInput{
		SnapshotHash: "0x1234567890abcdef",
		TradeBatchID: "test-batch",
		Snapshot: orderbookchecker.OrderbookSnapshot{
			SequenceNumber: 1,
			MarketID:       "TEST-MARKET",
			Orders: []orderbookchecker.Order{
				{
					ID:       "buy-1",
					Side:     "buy",
					Price:    big.NewInt(100),
					Quantity: big.NewInt(50),
					UserID:   "user1",
				},
				{
					ID:       "sell-1",
					Side:     "sell",
					Price:    big.NewInt(95),
					Quantity: big.NewInt(30),
					UserID:   "user2",
				},
			},
		},
		Trades: []orderbookchecker.Trade{
			{
				ID:          "trade-1",
				BuyOrderID:  "buy-1",
				SellOrderID: "sell-1",
				Price:       big.NewInt(95),
				Quantity:    big.NewInt(30),
			},
		},
	}

	payloadBytes, err := json.Marshal(taskInput)
	if err != nil {
		t.Fatalf("Failed to marshal task input: %v", err)
	}

	taskRequest := &performerV1.TaskRequest{
		TaskId:   []byte("test-task-id"),
		Payload:  payloadBytes,
		Metadata: []byte("test-metadata"),
	}

	err = taskWorker.ValidateTask(taskRequest)
	if err != nil {
		t.Errorf("ValidateTask failed: %v", err)
	}

	resp, err := taskWorker.HandleTask(taskRequest)
	if err != nil {
		t.Errorf("HandleTask failed: %v", err)
	}

	if resp == nil {
		t.Error("Expected response, got nil")
	} else {
		t.Logf("Response received with %d bytes", len(resp.Result))
	}
}
