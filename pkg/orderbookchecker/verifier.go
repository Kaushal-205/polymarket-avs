package orderbookchecker

import (
	"fmt"
	"go.uber.org/zap"
	"sort"
)

// OrderbookVerifier handles verification of orderbook snapshots against executed trades
type OrderbookVerifier struct {
	logger *zap.Logger
}

// NewOrderbookVerifier creates a new instance of OrderbookVerifier
func NewOrderbookVerifier(logger *zap.Logger) *OrderbookVerifier {
	return &OrderbookVerifier{
		logger: logger,
	}
}

// VerifySnapshot verifies that the executed trades are consistent with the orderbook snapshot
func (v *OrderbookVerifier) VerifySnapshot(trades []Trade, snapshot OrderbookSnapshot) (*VerificationResult, error) {
	v.logger.Sugar().Infow("Starting orderbook verification",
		"sequence_number", snapshot.SequenceNumber,
		"market_id", snapshot.MarketID,
		"total_trades", len(trades),
		"total_orders", len(snapshot.Orders),
	)

	// Build orderbook state from snapshot
	state, err := v.buildOrderbookState(snapshot.Orders)
	if err != nil {
		return &VerificationResult{
			Valid:        false,
			ErrorMessage: fmt.Sprintf("failed to build orderbook state: %v", err),
			TotalTrades:  len(trades),
		}, err
	}

	// Verify each trade
	result := &VerificationResult{
		Valid:       true,
		TotalTrades: len(trades),
	}

	for _, trade := range trades {
		if err := v.verifyTrade(trade, state); err != nil {
			v.logger.Sugar().Errorw("Trade verification failed",
				"trade_id", trade.ID,
				"error", err,
			)
			result.Valid = false
			result.FailedTrades = append(result.FailedTrades, trade.ID)
			if result.ErrorMessage == "" {
				result.ErrorMessage = fmt.Sprintf("trade %s failed: %v", trade.ID, err)
			}
		} else {
			result.VerifiedTrades++
		}
	}

	v.logger.Sugar().Infow("Verification completed",
		"valid", result.Valid,
		"verified_trades", result.VerifiedTrades,
		"failed_trades", len(result.FailedTrades),
	)

	return result, nil
}

// buildOrderbookState constructs the orderbook state from a list of orders
func (v *OrderbookVerifier) buildOrderbookState(orders []Order) (*OrderbookState, error) {
	state := &OrderbookState{
		BuyOrders:  make([]Order, 0),
		SellOrders: make([]Order, 0),
	}

	for _, order := range orders {
		if order.Side == "buy" {
			state.BuyOrders = append(state.BuyOrders, order)
		} else if order.Side == "sell" {
			state.SellOrders = append(state.SellOrders, order)
		} else {
			return nil, fmt.Errorf("invalid order side: %s", order.Side)
		}
	}

	// Sort buy orders by price (highest first), then by timestamp
	sort.Slice(state.BuyOrders, func(i, j int) bool {
		priceComp := state.BuyOrders[i].Price.Cmp(state.BuyOrders[j].Price)
		if priceComp == 0 {
			return state.BuyOrders[i].Timestamp.Before(state.BuyOrders[j].Timestamp)
		}
		return priceComp > 0
	})

	// Sort sell orders by price (lowest first), then by timestamp
	sort.Slice(state.SellOrders, func(i, j int) bool {
		priceComp := state.SellOrders[i].Price.Cmp(state.SellOrders[j].Price)
		if priceComp == 0 {
			return state.SellOrders[i].Timestamp.Before(state.SellOrders[j].Timestamp)
		}
		return priceComp < 0
	})

	return state, nil
}

// verifyTrade verifies a single trade against the orderbook state
func (v *OrderbookVerifier) verifyTrade(trade Trade, state *OrderbookState) error {
	// Find the buy and sell orders involved in this trade
	buyOrder, err := v.findOrderByID(trade.BuyOrderID, state.BuyOrders)
	if err != nil {
		return fmt.Errorf("buy order not found: %s", trade.BuyOrderID)
	}

	sellOrder, err := v.findOrderByID(trade.SellOrderID, state.SellOrders)
	if err != nil {
		return fmt.Errorf("sell order not found: %s", trade.SellOrderID)
	}

	// Verify price matching rules
	if err := v.verifyPriceMatching(trade, buyOrder, sellOrder); err != nil {
		return fmt.Errorf("price matching failed: %v", err)
	}

	// Verify quantity constraints
	if err := v.verifyQuantityConstraints(trade, buyOrder, sellOrder); err != nil {
		return fmt.Errorf("quantity constraints failed: %v", err)
	}

	// Verify time priority (simplified - assumes orders are already sorted)
	if err := v.verifyTimePriority(trade, buyOrder, sellOrder, state); err != nil {
		return fmt.Errorf("time priority failed: %v", err)
	}

	return nil
}

// findOrderByID finds an order by its ID in the given slice
func (v *OrderbookVerifier) findOrderByID(orderID string, orders []Order) (*Order, error) {
	for i := range orders {
		if orders[i].ID == orderID {
			return &orders[i], nil
		}
	}
	return nil, fmt.Errorf("order not found: %s", orderID)
}

// verifyPriceMatching verifies that the trade price is valid according to limit order rules
func (v *OrderbookVerifier) verifyPriceMatching(trade Trade, buyOrder, sellOrder *Order) error {
	// Buy order price must be >= trade price
	if buyOrder.Price.Cmp(trade.Price) < 0 {
		return fmt.Errorf("buy order price %s is less than trade price %s",
			buyOrder.Price.String(), trade.Price.String())
	}

	// Sell order price must be <= trade price
	if sellOrder.Price.Cmp(trade.Price) > 0 {
		return fmt.Errorf("sell order price %s is greater than trade price %s",
			sellOrder.Price.String(), trade.Price.String())
	}

	// Trade price should typically be the worse of the two order prices (price-time priority)
	// In practice, this is usually the sell order price for a buy-initiated trade
	return nil
}

// verifyQuantityConstraints verifies that the trade quantity doesn't exceed order quantities
func (v *OrderbookVerifier) verifyQuantityConstraints(trade Trade, buyOrder, sellOrder *Order) error {
	// Trade quantity must not exceed buy order quantity
	if trade.Quantity.Cmp(buyOrder.Quantity) > 0 {
		return fmt.Errorf("trade quantity %s exceeds buy order quantity %s",
			trade.Quantity.String(), buyOrder.Quantity.String())
	}

	// Trade quantity must not exceed sell order quantity
	if trade.Quantity.Cmp(sellOrder.Quantity) > 0 {
		return fmt.Errorf("trade quantity %s exceeds sell order quantity %s",
			trade.Quantity.String(), sellOrder.Quantity.String())
	}

	return nil
}

// verifyTimePriority verifies that the matched orders respect price-time priority
func (v *OrderbookVerifier) verifyTimePriority(trade Trade, buyOrder, sellOrder *Order, state *OrderbookState) error {
	// For buy orders: check that no earlier buy order at same or better price exists
	for _, order := range state.BuyOrders {
		if order.ID == buyOrder.ID {
			break // We've reached the matched order, so priority is respected
		}
		if order.Price.Cmp(buyOrder.Price) >= 0 && order.Timestamp.Before(buyOrder.Timestamp) {
			// There's an earlier order at same or better price that should have been matched first
			return fmt.Errorf("buy order %s has priority over %s (price: %s vs %s, time: %v vs %v)",
				order.ID, buyOrder.ID, order.Price.String(), buyOrder.Price.String(),
				order.Timestamp, buyOrder.Timestamp)
		}
	}

	// For sell orders: check that no earlier sell order at same or better price exists
	for _, order := range state.SellOrders {
		if order.ID == sellOrder.ID {
			break // We've reached the matched order, so priority is respected
		}
		if order.Price.Cmp(sellOrder.Price) <= 0 && order.Timestamp.Before(sellOrder.Timestamp) {
			// There's an earlier order at same or better price that should have been matched first
			return fmt.Errorf("sell order %s has priority over %s (price: %s vs %s, time: %v vs %v)",
				order.ID, sellOrder.ID, order.Price.String(), sellOrder.Price.String(),
				order.Timestamp, sellOrder.Timestamp)
		}
	}

	return nil
}
