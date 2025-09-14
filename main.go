package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"
	

    

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/joho/godotenv"
)

type HedgingBot struct {
	spotClient    *binance.Client
	futuresClient *futures.Client
	
	// Strategy parameters
	minProfitThreshold float64  // Minimum profit after fees
	maxPositionSize    float64  // Maximum position size
	spotFeeRate        float64  // Spot trading fee
	futuresFeeRate     float64  // Futures trading fee
	slippageTolerance  float64  // Expected slippage %
	
	// Position tracking 
	// currentSpotPosition    float64
	// currentFuturesPosition float64
}

func NewAdvancedBot(apiKey, secretKey string, testnet bool) *HedgingBot {
	if testnet {
		binance.UseTestnet = true
		futures.UseTestnet = true
		log.Println("Using TESTNET environment")
	}
	
	return &HedgingBot{
		spotClient:         binance.NewClient(apiKey, secretKey),
		futuresClient:      futures.NewClient(apiKey, secretKey),
		minProfitThreshold: 0.0005,  // 0.05% minimum profit
		maxPositionSize:    0.1,     // Max 0.1 BTC position
		spotFeeRate:        0.001,   // 0.1% spot fee
		futuresFeeRate:     0.0004,  // 0.04% futures fee
		slippageTolerance:  0.0002,  // 0.02% slippage tolerance
	}
}

// Strategy 1: Funding Rate Arbitrage with Fee Calculation
func (bot *HedgingBot) FundingRateArbitrage() error {
	fundingRate, err := bot.GetFundingRate()
	if err != nil {
		return err
	}
	
	// Calculate 8-hour funding profit/cost
	fundingProfit := math.Abs(fundingRate)
	
	// Calculate total trading costs (round trip)
	totalFees := (bot.spotFeeRate + bot.futuresFeeRate) * 2 // Buy + Sell on both
	slippageCost := bot.slippageTolerance * 2
	totalCost := totalFees + slippageCost
	
	netProfit := fundingProfit - totalCost
	
	log.Printf("Funding Rate: %.6f (%.4f%%), Net Profit Potential: %.6f (%.4f%%)", 
		fundingRate, fundingRate*100, netProfit, netProfit*100)
	
	if netProfit > bot.minProfitThreshold {
		btcBalance, _ := bot.GetBTCBalance()
		usdtBalance, _ := bot.GetUSDTBalance()
		
		// Calculate optimal position size
		maxSizeFromBTC := btcBalance * 0.8  // Use 80% of BTC
		
		btcPrice, err := bot.GetBTCPrice()
	    if err != nil {
        return err
        }
        maxSizeFromUSDT := usdtBalance * 0.8 / btcPrice
		positionSize := math.Min(math.Min(maxSizeFromBTC, maxSizeFromUSDT), bot.maxPositionSize)
		
		if positionSize > 0.001 { // Minimum 0.001 BTC
			if fundingRate > 0 {
				// Positive funding: Short futures, Long spot
				log.Printf("Executing FUNDING ARBITRAGE: Short futures, Long spot (%.6f BTC)", positionSize)
				return bot.ExecuteFundingArbitrage(positionSize, "SHORT_FUTURES_LONG_SPOT")
			} else {
				// Negative funding: Long futures, Short spot
				log.Printf("Executing FUNDING ARBITRAGE: Long futures, Short spot (%.6f BTC)", positionSize)
				return bot.ExecuteFundingArbitrage(positionSize, "LONG_FUTURES_SHORT_SPOT")
			}
		}
	}
	
	return nil
}

// Strategy 2: Basis Trading (Spot-Futures Price Difference)
func (bot *HedgingBot) BasisTrading() error {
	spotPrice, err := bot.GetSpotPrice()
	if err != nil {
		return err
	}
	
	futuresPrice, err := bot.GetFuturesPrice()
	if err != nil {
		return err
	}
	
	basis := (futuresPrice - spotPrice) / spotPrice
	
	// Calculate costs
	totalCost := (bot.spotFeeRate + bot.futuresFeeRate) * 2 + bot.slippageTolerance * 2
	netBasisProfit := math.Abs(basis) - totalCost
	
	log.Printf("Spot: $%.2f, Futures: $%.2f, Basis: %.6f (%.4f%%), Net Profit: %.4f%%", 
		spotPrice, futuresPrice, basis, basis*100, netBasisProfit*100)
	
	if netBasisProfit > bot.minProfitThreshold {
		positionSize := bot.CalculateOptimalSize()
		
		if basis > 0 {
			// Futures premium: Short futures, Long spot
			log.Printf("Executing BASIS TRADE: Futures premium detected (%.6f BTC)", positionSize)
			return bot.ExecuteBasisTrade(positionSize, "SHORT_FUTURES_LONG_SPOT")
		} else {
			// Spot premium: Long futures, Short spot
			log.Printf("Executing BASIS TRADE: Spot premium detected (%.6f BTC)", positionSize)
			return bot.ExecuteBasisTrade(positionSize, "LONG_FUTURES_SHORT_SPOT")
		}
	}
	
	return nil
}

// Strategy 3: Delta Neutral Portfolio Rebalancing
func (bot *HedgingBot) DeltaNeutralRebalancing() error {
	btcBalance, _ := bot.GetBTCBalance()
	futuresPosition, _ := bot.GetFuturesPosition()
	
	// Calculate current delta exposure
	netExposure := btcBalance + futuresPosition
	
	log.Printf("Current Exposure - Spot: %.6f, Futures: %.6f, Net: %.6f", 
		btcBalance, futuresPosition, netExposure)
	
	// Target is delta neutral (net exposure = 0)
	if math.Abs(netExposure) > 0.01 { // Rebalance if exposure > 0.01 BTC
		rebalanceAmount := netExposure / 2 // Split the rebalancing
		
		// Calculate if rebalancing is profitable after costs
		rebalanceCost := math.Abs(rebalanceAmount) * (bot.futuresFeeRate + bot.slippageTolerance)
		
		if rebalanceCost < math.Abs(netExposure) * 0.001 { // If cost < 0.1% of exposure
			log.Printf("Rebalancing portfolio: %.6f BTC", rebalanceAmount)
			
			if netExposure > 0 {
				// Too much long exposure: Short futures
				return bot.PlaceFuturesOrder("SELL", math.Abs(rebalanceAmount))
			} else {
				// Too much short exposure: Long futures
				return bot.PlaceFuturesOrder("BUY", math.Abs(rebalanceAmount))
			}
		}
	}
	
	return nil
}

// Strategy 4: Volatility-Based Dynamic Hedging
func (bot *HedgingBot) VolatilityHedging() error {
	volatility, err := bot.CalculateVolatility()
	if err != nil {
		return err
	}
	
	// Adjust hedge ratio based on volatility
	baseHedgeRatio := 0.8
	volatilityAdjustment := math.Min(volatility * 10, 0.2) // Max 20% adjustment
	hedgeRatio := baseHedgeRatio + volatilityAdjustment
	
	btcBalance, _ := bot.GetBTCBalance()
	targetHedge := btcBalance * hedgeRatio
	currentHedge, _ := bot.GetFuturesPosition()
	
	hedgeDifference := targetHedge - math.Abs(currentHedge)
	
	log.Printf("Volatility: %.4f%%, Target Hedge: %.6f, Current: %.6f, Difference: %.6f", 
		volatility*100, targetHedge, currentHedge, hedgeDifference)
	
	if math.Abs(hedgeDifference) > 0.01 {
		adjustmentCost := math.Abs(hedgeDifference) * (bot.futuresFeeRate + bot.slippageTolerance)
		expectedBenefit := math.Abs(hedgeDifference) * volatility * 0.5 // Estimated benefit
		
		if expectedBenefit > adjustmentCost {
			if hedgeDifference > 0 {
				// Need more hedge
				return bot.PlaceFuturesOrder("SELL", hedgeDifference)
			} else {
				// Need less hedge
				return bot.PlaceFuturesOrder("BUY", math.Abs(hedgeDifference))
			}
		}
	}
	
	return nil
}

// Execute funding rate arbitrage
func (bot *HedgingBot) ExecuteFundingArbitrage(size float64, strategy string) error {
	switch strategy {
	case "SHORT_FUTURES_LONG_SPOT":
		// Buy spot, sell futures
		if err := bot.PlaceSpotOrderWithSlippage("BUY", size); err != nil {
			return err
		}
		return bot.PlaceFuturesOrderWithSlippage("SELL", size)
		
	case "LONG_FUTURES_SHORT_SPOT":
		// Sell spot, buy futures
		if err := bot.PlaceSpotOrderWithSlippage("SELL", size); err != nil {
			return err
		}
		return bot.PlaceFuturesOrderWithSlippage("BUY", size)
	}
	return nil
}

// Execute basis trade
func (bot *HedgingBot) ExecuteBasisTrade(size float64, strategy string) error {
	return bot.ExecuteFundingArbitrage(size, strategy) // Same execution logic
}

// Enhanced order placement with slippage protection
func (bot *HedgingBot) PlaceSpotOrderWithSlippage(side string, quantity float64) error {
	currentPrice, err := bot.GetSpotPrice()
	if err != nil {
		return err
	}
	
	// Calculate limit price with slippage protection
	var limitPrice float64
	if side == "BUY" {
		limitPrice = currentPrice * (1 + bot.slippageTolerance)
	} else {
		limitPrice = currentPrice * (1 - bot.slippageTolerance)
	}
	
	orderSide := binance.SideTypeBuy
	if side == "SELL" {
		orderSide = binance.SideTypeSell
	}
	
	_, err = bot.spotClient.NewCreateOrderService().
		Symbol("BTCUSDT").
		Side(orderSide).
		Type(binance.OrderTypeLimit).
		TimeInForce(binance.TimeInForceTypeIOC). // Immediate or Cancel
		Quantity(fmt.Sprintf("%.6f", quantity)).
		Price(fmt.Sprintf("%.2f", limitPrice)).
		Do(context.Background())
	
	if err != nil {
		log.Printf("Spot order failed, trying market order: %v", err)
		// Fallback to market order
		return bot.PlaceSpotOrder(side, quantity)
	}
	
	log.Printf("Spot %s order: %.6f BTC at $%.2f (slippage protected)", side, quantity, limitPrice)
	return nil
}

func (bot *HedgingBot) PlaceFuturesOrderWithSlippage(side string, quantity float64) error {
	currentPrice, err := bot.GetFuturesPrice()
	if err != nil {
		return err
	}
	
	var limitPrice float64
	if side == "BUY" {
		limitPrice = currentPrice * (1 + bot.slippageTolerance)
	} else {
		limitPrice = currentPrice * (1 - bot.slippageTolerance)
	}
	
	orderSide := futures.SideTypeBuy
	if side == "SELL" {
		orderSide = futures.SideTypeSell
	}
	
	_, err = bot.futuresClient.NewCreateOrderService().
		Symbol("BTCUSDT").
		Side(orderSide).
		Type(futures.OrderTypeLimit).
		TimeInForce(futures.TimeInForceTypeIOC).
		Quantity(fmt.Sprintf("%.3f", quantity)).
		Price(fmt.Sprintf("%.2f", limitPrice)).
		Do(context.Background())
	
	if err != nil {
		log.Printf("Futures order failed, trying market order: %v", err)
		return bot.PlaceFuturesOrder(side, quantity)
	}
	
	log.Printf("Futures %s order: %.3f BTC at $%.2f (slippage protected)", side, quantity, limitPrice)
	return nil
}

// Calculate optimal position size based on available capital and risk
func (bot *HedgingBot) CalculateOptimalSize() float64 {
	btcBalance, _ := bot.GetBTCBalance()
	usdtBalance, _ := bot.GetUSDTBalance()
	btcPrice, _ := bot.GetBTCPrice()
	
	maxFromBTC := btcBalance * 0.8
	maxFromUSDT := (usdtBalance * 0.8) / btcPrice
	
	optimalSize := math.Min(math.Min(maxFromBTC, maxFromUSDT), bot.maxPositionSize)
	return math.Max(optimalSize, 0.001) // Minimum 0.001 BTC
}

// Calculate volatility based on recent price movements
func (bot *HedgingBot) CalculateVolatility() (float64, error) {
	klines, err := bot.spotClient.NewKlinesService().
		Symbol("BTCUSDT").
		Interval("1m").
		Limit(20).
		Do(context.Background())
	
	if err != nil {
		return 0, err
	}
	
	var returns []float64
	for i := 1; i < len(klines); i++ {
		prevClose, _ := strconv.ParseFloat(klines[i-1].Close, 64)
		currClose, _ := strconv.ParseFloat(klines[i].Close, 64)
		ret := (currClose - prevClose) / prevClose
		returns = append(returns, ret)
	}
	
	// Calculate standard deviation
	var sum, mean float64
	for _, ret := range returns {
		sum += ret
	}
	mean = sum / float64(len(returns))
	
	var variance float64
	for _, ret := range returns {
		variance += (ret - mean) * (ret - mean)
	}
	variance /= float64(len(returns))
	
	return math.Sqrt(variance), nil
}

// Advanced main loop with multiple strategies
func (bot *HedgingBot) RunAdvancedHedging() {
	log.Println("Starting advanced hedging bot with multiple strategies...")
	
	strategies := []func() error{
		bot.FundingRateArbitrage,
		bot.BasisTrading,
		bot.DeltaNeutralRebalancing,
		bot.VolatilityHedging,
	}
	
	strategyNames := []string{
		"Funding Rate Arbitrage",
		"Basis Trading", 
		"Delta Neutral Rebalancing",
		"Volatility Hedging",
	}
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	strategyIndex := 0
	for {
		select {
		case <-ticker.C:
			currentStrategy := strategies[strategyIndex]
			strategyName := strategyNames[strategyIndex]
			
			log.Printf("=== Executing Strategy: %s ===", strategyName)
			
			if err := currentStrategy(); err != nil {
				log.Printf("Strategy %s failed: %v", strategyName, err)
			}
			
			// Rotate to next strategy
			strategyIndex = (strategyIndex + 1) % len(strategies)
			
			// Print portfolio summary every 4 strategies (2 minutes)
			if strategyIndex == 0 {
				bot.PrintPortfolioSummary()
			}
		}
	}
}

func (bot *HedgingBot) PrintPortfolioSummary() {
	btc, _ := bot.GetBTCBalance()
	usdt, _ := bot.GetUSDTBalance()
	futuresPos, _ := bot.GetFuturesPosition()
	btcPrice, _ := bot.GetBTCPrice()
	
	totalValue := (btc * btcPrice) + usdt + (futuresPos * btcPrice)
	
	log.Printf("=== Portfolio Summary ===")
	log.Printf("Spot BTC: %.6f ($%.2f)", btc, btc*btcPrice)
	log.Printf("USDT: %.2f", usdt)
	log.Printf("Futures Position: %.6f ($%.2f)", futuresPos, futuresPos*btcPrice)
	log.Printf("Total Portfolio Value: $%.2f", totalValue)
	log.Printf("Net Exposure: %.6f BTC", btc+futuresPos)
}

// Helper functions (add these to your existing code)
func (bot *HedgingBot) GetUSDTBalance() (float64, error) {
	account, err := bot.spotClient.NewGetAccountService().Do(context.Background())
	if err != nil {
		return 0, err
	}
	
	for _, balance := range account.Balances {
		if balance.Asset == "USDT" {
			usdt, _ := strconv.ParseFloat(balance.Free, 64)
			return usdt, nil
		}
	}
	return 0, nil
}

func (bot *HedgingBot) GetSpotPrice() (float64, error) {
	ticker, err := bot.spotClient.NewListPricesService().Symbol("BTCUSDT").Do(context.Background())
	if err != nil {
		return 0, err
	}
	price, _ := strconv.ParseFloat(ticker[0].Price, 64)
	return price, nil
}

func (bot *HedgingBot) GetFuturesPrice() (float64, error) {
	ticker, err := bot.futuresClient.NewListPricesService().Symbol("BTCUSDT").Do(context.Background())
	if err != nil {
		return 0, err
	}
	price, _ := strconv.ParseFloat(ticker[0].Price, 64)
	return price, nil
}

func (bot *HedgingBot) GetBTCPrice() (float64, error) {
	return bot.GetSpotPrice()
}

func (bot *HedgingBot) GetFuturesPosition() (float64, error) {
	positions, err := bot.futuresClient.NewGetPositionRiskService().Do(context.Background())
	if err != nil {
		return 0, err
	}
	
	for _, pos := range positions {
		if pos.Symbol == "BTCUSDT" {
			size, _ := strconv.ParseFloat(pos.PositionAmt, 64)
			return size, nil
		}
	}
	return 0, nil
}

// Add your existing helper functions here...
func (bot *HedgingBot) GetFundingRate() (float64, error) {
	// Use Binance REST API directly
	var url string
	if futures.UseTestnet {
		url = "https://testnet.binancefuture.com/fapi/v1/premiumIndex?symbol=BTCUSDT"
	} else {
		url = "https://fapi.binance.com/fapi/v1/premiumIndex?symbol=BTCUSDT"
	}
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		EstimatedSettlePrice string `json:"estimatedSettlePrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to parse JSON: %v", err)
	}
	
	fundingRate, err := strconv.ParseFloat(result.LastFundingRate, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse funding rate: %v", err)
	}
	
	log.Printf("✅ Funding Rate: %.6f (%.4f%%)", fundingRate, fundingRate*100)
	return fundingRate, nil
}

func (bot *HedgingBot) GetBTCBalance() (float64, error) {
	account, err := bot.spotClient.NewGetAccountService().Do(context.Background())
	if err != nil {
		return 0, err
	}
	
	for _, balance := range account.Balances {
		if balance.Asset == "BTC" {
			btc, _ := strconv.ParseFloat(balance.Free, 64)
			return btc, nil
		}
	}
	return 0, nil
}

func (bot *HedgingBot) PlaceSpotOrder(side string, quantity float64) error {
	orderSide := binance.SideTypeBuy
	if side == "SELL" {
		orderSide = binance.SideTypeSell
	}
	
	_, err := bot.spotClient.NewCreateOrderService().
		Symbol("BTCUSDT").
		Side(orderSide).
		Type(binance.OrderTypeMarket).
		Quantity(fmt.Sprintf("%.6f", quantity)).
		Do(context.Background())
	
	if err != nil {
		return err
	}
	
	log.Printf("Spot %s order: %.6f BTC", side, quantity)
	return nil
}

func (bot *HedgingBot) PlaceFuturesOrder(side string, quantity float64) error {
	orderSide := futures.SideTypeBuy
	if side == "SELL" {
		orderSide = futures.SideTypeSell
	}
	
	_, err := bot.futuresClient.NewCreateOrderService().
		Symbol("BTCUSDT").
		Side(orderSide).
		Type(futures.OrderTypeMarket).
		Quantity(fmt.Sprintf("%.3f", quantity)).
		Do(context.Background())
	
	if err != nil {
		return err
	}
	
	log.Printf("Futures %s order: %.3f BTC", side, quantity)
	return nil
}

func getEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Environment variable %s is not set", key)
	}
	return value
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}
	
	apiKey := getEnv("BINANCE_API_KEY")
	secretKey := getEnv("BINANCE_SECRET_KEY")
	testnet := getEnv("BINANCE_TESTNET") == "true"
	
	bot := NewAdvancedBot(apiKey, secretKey, testnet)
	bot.RunAdvancedHedging()
}