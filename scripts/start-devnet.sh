#!/bin/bash
set -e

echo "🚀 Starting Polymarket AVS DevNet Deployment"
echo "============================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
print_status "Checking prerequisites..."

if ! command -v docker &> /dev/null; then
    print_error "Docker is not installed"
    exit 1
fi

if ! docker compose version &> /dev/null; then
    print_error "Docker Compose is not installed"
    exit 1
fi

print_success "Prerequisites check passed"

# Build our AVS performer
print_status "Building Polymarket AVS performer..."
make build
print_success "AVS performer built successfully"

# Build container
print_status "Building AVS container..."
docker build -t polymarket-avs:latest .
print_success "Container built successfully"

# Create network if it doesn't exist
print_status "Creating Docker network..."
docker network create hourglass-network 2>/dev/null || print_warning "Network already exists"

# Start Anvil (local Ethereum node)
print_status "Starting local Ethereum node (Anvil)..."
docker run -d \
    --name anvil-l1 \
    --network hourglass-network \
    -p 8545:8545 \
    ghcr.io/foundry-rs/foundry:latest \
    anvil --host 0.0.0.0 --port 8545 --chain-id 31337 \
    --accounts 10 --balance 10000 \
    || print_warning "Anvil L1 already running"

# Start L2 node (for testing)
print_status "Starting L2 node..."
docker run -d \
    --name anvil-l2 \
    --network hourglass-network \
    -p 8547:8547 \
    ghcr.io/foundry-rs/foundry:latest \
    anvil --host 0.0.0.0 --port 8547 --chain-id 84532 \
    --accounts 10 --balance 10000 \
    || print_warning "Anvil L2 already running"

# Wait for nodes to start
print_status "Waiting for nodes to start..."
sleep 5

# Deploy contracts (if available)
print_status "Deploying contracts..."
if [ -d ".devkit/contracts" ]; then
    cd .devkit/contracts
    if [ -f "foundry.toml" ]; then
        forge script script/Deploy.s.sol --rpc-url http://localhost:8545 --broadcast --private-key 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 || print_warning "Contract deployment failed"
    fi
    cd ../..
fi

# Start our AVS performer
print_status "Starting Polymarket AVS performer..."
docker run -d \
    --name polymarket-avs-performer \
    --network hourglass-network \
    -p 8080:8080 \
    -e LOG_LEVEL=info \
    -e LOG_FORMAT=json \
    polymarket-avs:latest \
    || print_warning "AVS performer already running"

# Show running containers
print_status "Checking running services..."
docker ps --filter "network=hourglass-network" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

print_success "DevNet deployment completed!"
echo ""
echo "📋 Service Endpoints:"
echo "  - L1 RPC: http://localhost:8545"
echo "  - L2 RPC: http://localhost:8547"
echo "  - AVS Performer: http://localhost:8080"
echo ""
echo "🔧 Next steps:"
echo "  1. Run 'make demo' to test the verification flow"
echo "  2. Submit tasks using the demo CLI"
echo "  3. Monitor logs with 'docker logs -f polymarket-avs-performer'"
echo ""
echo "🛑 To stop the devnet: ./scripts/stop-devnet.sh" 