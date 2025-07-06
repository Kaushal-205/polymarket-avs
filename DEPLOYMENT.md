# Polymarket AVS DevNet Deployment Guide

## Overview

This guide covers the deployment and testing of the Polymarket AVS on a local development network (DevNet).

## Prerequisites

- Docker 20.10+
- Docker Compose v2.0+
- Go 1.21+
- Make

## Quick Start

### 1. Start the DevNet

```bash
make devnet-start
```

This will:
- Build the Polymarket AVS performer binary
- Create a Docker container for the AVS
- Start local Ethereum L1 node (Anvil) on port 8545
- Start local L2 node (Anvil) on port 8547  
- Start the AVS performer gRPC server on port 8080
- Create a Docker network for all services

### 2. Verify Deployment

```bash
make devnet-status
```

Expected output:
```
📊 DevNet Status:
NAMES                      STATUS          PORTS
polymarket-avs-performer   Up X seconds    0.0.0.0:8080->8080/tcp
anvil-l2                   Up X seconds    0.0.0.0:8547->8547/tcp
anvil-l1                   Up X seconds    0.0.0.0:8545->8545/tcp
```

### 3. Run End-to-End Demo

```bash
make demo
```

This will:
- Generate a sample orderbook snapshot
- Submit a verification task
- Simulate the AVS verification process
- Display the results

## Service Endpoints

| Service | Endpoint | Description |
|---------|----------|-------------|
| L1 Node | `http://localhost:8545` | Local Ethereum node |
| L2 Node | `http://localhost:8547` | Local L2 node |
| AVS Performer | `localhost:8080` | gRPC server for task processing |

## Available Commands

### DevNet Management
```bash
make devnet-start    # Start all services
make devnet-stop     # Stop all services  
make devnet-status   # Check service status
make devnet-logs     # View AVS performer logs
```

### Demo and Testing
```bash
make demo            # Run full verification demo
make demo-publish    # Generate sample snapshot
make demo-watch      # Watch for new snapshots
```

### Development
```bash
make build           # Build AVS performer
make build-all       # Build all components
make test            # Run all tests
make clean           # Clean build artifacts
```

## Architecture

```
┌─────────────────────┐    ┌─────────────────────┐
│   Snapshot          │    │   Task Submitter    │
│   Publisher         │───▶│   (Aggregator)      │
│                     │    │                     │
└─────────────────────┘    └─────────────────────┘
                                       │
                                       ▼
┌─────────────────────┐    ┌─────────────────────┐
│   L1 Node           │    │   AVS Performer     │
│   (Anvil)           │◀───│   (Verification)    │
│   Port: 8545        │    │   Port: 8080        │
└─────────────────────┘    └─────────────────────┘
                                       │
┌─────────────────────┐                │
│   L2 Node           │◀───────────────┘
│   (Anvil)           │
│   Port: 8547        │
└─────────────────────┘
```

## Sample Verification Flow

1. **Snapshot Generation**: Create orderbook snapshot with orders and trades
2. **Task Submission**: Package snapshot into verification task
3. **AVS Processing**: Verify trade execution against orderbook rules
4. **Result Aggregation**: Collect and validate results from operators

## Demo Output Example

```
🚀 Starting Polymarket AVS Full Demo
===================================

📸 Step 1: Publishing orderbook snapshot...
✅ Snapshot published:
   - Sequence: 1
   - Market: TRUMP-2024-WIN
   - Orders: 4
   - Trades: 1
   - Merkle Root: 0xd4b202c5649d39b2287d1d2ac9c86965a035b018c22257363e224389497bbfff

👁️  Step 2: Starting task submitter (watching for snapshots)...
✅ Task submitted:
   - Task ID: task-1-1751777304
   - Batch ID: batch-1
   - Snapshot Hash: 0xd4b202c5649d39b2287d1d2ac9c86965a035b018c22257363e224389497bbfff
   - Trades: 1

🔍 Step 4: Simulating AVS verification...
✅ Verification completed:
   - Valid: true
   - Verified Trades: 1/1

🎉 Demo completed successfully!
```

## Troubleshooting

### Services Not Starting
```bash
# Check Docker daemon
sudo systemctl status docker

# Check port availability
netstat -tlnp | grep -E '(8080|8545|8547)'

# View service logs
docker logs anvil-l1
docker logs anvil-l2
docker logs polymarket-avs-performer
```

### Demo Failures
```bash
# Clean up and restart
make devnet-stop
make clean
make devnet-start

# Check demo files
ls -la demo-snapshots/
```

### Build Issues
```bash
# Clean Go modules
go clean -modcache
go mod tidy

# Rebuild everything
make clean
make build-all
```

## Next Steps

1. **Contract Deployment**: Deploy AVS contracts to L1/L2 networks
2. **Operator Registration**: Register operators with the AVS
3. **Production Testing**: Test with real Polymarket data
4. **Monitoring Setup**: Configure metrics and alerting
5. **Security Audit**: Review smart contracts and off-chain components

## Production Considerations

- **Scalability**: Handle thousands of verification tasks per second
- **Security**: Implement proper key management and access controls
- **Monitoring**: Set up comprehensive logging and metrics
- **High Availability**: Deploy redundant operators and infrastructure
- **Economic Security**: Configure proper slashing and rewards mechanisms

## Support

For issues or questions:
- Check the logs: `make devnet-logs`
- Review the demo output for errors
- Ensure all prerequisites are installed
- Verify Docker network connectivity 