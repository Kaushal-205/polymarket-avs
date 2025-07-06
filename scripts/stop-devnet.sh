#!/bin/bash
set -e

echo "🛑 Stopping Polymarket AVS DevNet"
echo "================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Stop and remove containers
print_status "Stopping AVS containers..."
docker stop polymarket-avs-performer 2>/dev/null || print_warning "AVS performer not running"
docker rm polymarket-avs-performer 2>/dev/null || print_warning "AVS performer container not found"

print_status "Stopping Anvil nodes..."
docker stop anvil-l1 2>/dev/null || print_warning "Anvil L1 not running"
docker rm anvil-l1 2>/dev/null || print_warning "Anvil L1 container not found"

docker stop anvil-l2 2>/dev/null || print_warning "Anvil L2 not running"
docker rm anvil-l2 2>/dev/null || print_warning "Anvil L2 container not found"

# Remove network
print_status "Removing Docker network..."
docker network rm hourglass-network 2>/dev/null || print_warning "Network not found"

# Clean up demo snapshots
print_status "Cleaning up demo files..."
rm -rf demo-snapshots 2>/dev/null || true

print_success "DevNet stopped and cleaned up!"
echo ""
echo "🔄 To restart the devnet: ./scripts/start-devnet.sh" 