# Polymarket AVS - Verifiable Orderbook Matching

## Overview

This AVS (Actively Validated Service) provides verifiable orderbook matching for centralized limit order book (CLOB) systems like Polymarket. It ensures that off-chain order matching is honest and follows proper price-time priority rules.

## Problem Statement

Polymarket's CLOB order matching is centralized:
- Orders are submitted to Polymarket's backend
- Matched off-chain by a centralized engine
- Settlement happens on-chain

This creates a trust issue: if the backend misbehaves (skips orders, front-runs, reorders for MEV), users can't easily prove it.

## Solution

This AVS verifies that on-chain executed trades are consistent with published off-chain orderbook snapshots by:

1. **Consuming** off-chain published orderbook logs and on-chain settlement data
2. **Replaying** the matching logic for those orders
3. **Verifying** that each on-chain fill has a matching off-chain order in the correct sequence & price
4. **Challenging** settlements when mismatches are found

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Polymarket    │    │   Orderbook     │    │   On-chain      │
│   CLOB Backend  │───▶│   Snapshots     │───▶│   Settlement    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   AVS Verifier  │───▶│   Challenge     │
                       │   (This AVS)    │    │   Contract      │
                       └─────────────────┘    └─────────────────┘
```

## Components

### 1. Go Orderbook Verifier (`pkg/orderbookchecker/`)

- **Types**: Data structures for orders, trades, and snapshots
- **Verifier**: Core logic to verify trade consistency
- **Features**:
  - Price matching validation
  - Quantity constraint checking
  - Time priority verification
  - Comprehensive error reporting

### 2. AVS Task Worker (`cmd/main.go`)

- **ValidateTask**: Validates incoming verification requests
- **HandleTask**: Performs orderbook verification and returns results
- **Integration**: Uses the Go verifier library

### 3. Settlement Verifier Contract (`contracts/src/l1-contracts/SettlementVerifier.sol`)

- **Settlement Registration**: Operators register settlements with stake
- **Challenge System**: Authorized challengers can dispute settlements
- **Slashing Mechanism**: Fraudulent operators lose stake
- **Time-based Resolution**: 7-day challenge period

## Data Format

### Task Input
```json
{
  "snapshot_hash": "0x1234...",
  "trade_batch_id": "batch-001",
  "snapshot": {
    "sequence_number": 1,
    "timestamp": "2024-01-15T10:00:00Z",
    "market_id": "TRUMP-2024-WIN",
    "orders": [
      {
        "id": "order-buy-001",
        "side": "buy",
        "price": "520000000000000000",
        "quantity": "1000000000000000000",
        "timestamp": "2024-01-15T09:58:00Z",
        "user_id": "user-alice"
      }
    ],
    "merkle_root": "0xabcdef...",
    "prev_hash": "0x987654..."
  },
  "trades": [
    {
      "id": "trade-001",
      "buy_order_id": "order-buy-001",
      "sell_order_id": "order-sell-002",
      "price": "525000000000000000",
      "quantity": "800000000000000000",
      "timestamp": "2024-01-15T10:00:00Z",
      "tx_hash": "0xdef123...",
      "block_number": 12345678
    }
  ]
}
```

### Task Output
```json
{
  "verification_result": {
    "valid": true,
    "verified_trades": 1,
    "total_trades": 1,
    "failed_trades": [],
    "error_message": ""
  },
  "snapshot_hash": "0x1234...",
  "trade_batch_id": "batch-001",
  "verified_at": "2024-01-15T10:00:05Z",
  "verifier_version": "1.0.0"
}
```

## Getting Started

### Prerequisites

- Go 1.23.6+
- Foundry (for Solidity contracts)
- Docker (for DevKit)

### Setup

1. **Clone and install dependencies**:
```bash
git clone <repository>
cd polymarket-avs
go mod tidy
```

2. **Run tests**:
```bash
# Go tests
go test ./pkg/orderbookchecker/... -v

# Solidity tests
forge test --match-path contracts/test/SettlementVerifier.t.sol -v
```

3. **Build the AVS**:
```bash
go build -o polymarket-avs ./cmd/
```

### Using DevKit

1. **Build the AVS**:
```bash
devkit avs build
```

2. **Start local devnet**:
```bash
devkit avs devnet start
```

3. **Test with sample data**:
```bash
devkit avs call --signature="(string)" args='("examples/sample_task.json")'
```

## Verification Logic

The AVS performs the following checks:

1. **Price Matching**: 
   - Buy order price ≥ trade price
   - Sell order price ≤ trade price

2. **Quantity Constraints**:
   - Trade quantity ≤ buy order quantity
   - Trade quantity ≤ sell order quantity

3. **Time Priority**:
   - Earlier orders at same price level are matched first
   - Proper price-time priority is maintained

4. **Order Existence**:
   - All referenced orders exist in the snapshot
   - No phantom orders are matched

## Challenge Process

1. **Settlement Registration**: Operators register settlements with stake
2. **Challenge Period**: 7-day window for challenges
3. **Proof Submission**: Challengers submit fraud proofs
4. **Resolution**: Contract owner resolves challenges
5. **Slashing**: Fraudulent operators lose 50% of stake

## Security Considerations

- **Stake Requirements**: Minimum 1 ETH stake for settlement registration
- **Authorization**: Only authorized addresses can submit challenges
- **Time Limits**: Challenge period prevents indefinite disputes
- **Slashing**: Economic incentive against fraudulent behavior

## Testing

### Unit Tests
```bash
go test ./pkg/orderbookchecker/... -v
```

### Integration Tests
```bash
go test ./cmd/... -v
```

### Contract Tests
```bash
forge test -v
```

## Deployment

### Local Development
```bash
devkit avs devnet start
devkit avs call --signature="(string)" args='("examples/sample_task.json")'
```

### Production
1. Deploy SettlementVerifier contract
2. Register AVS with EigenLayer
3. Authorize challenge addresses
4. Start operator nodes

## Future Enhancements

- **Automated Challenge Resolution**: Use cryptographic proofs instead of manual resolution
- **Multiple Market Support**: Handle multiple markets simultaneously  
- **Advanced Matching Logic**: Support for more complex order types
- **Gas Optimization**: Reduce on-chain verification costs
- **Decentralized Snapshots**: IPFS-based snapshot storage

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

MIT License - see LICENSE file for details. 