package simulators

import (
	"context"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	v4quoter "github.com/diadata-org/lumina-library/contracts/uniswapv4/V4Quoter"
	poolManager "github.com/diadata-org/lumina-library/contracts/uniswapv4/poolManager"
	"github.com/diadata-org/lumina-library/models"
	"github.com/diadata-org/lumina-library/utils"
)

type UniswapV4Simulator struct {
	restClient    *ethclient.Client
	luminaClient  *ethclient.Client
	quoter        *v4quoter.V4Quoter
	poolManager   *poolManager.PoolManager
	exchangepairs []models.ExchangePair
	priceMap      map[models.Asset]models.AssetQuotation
}

var (
	restUrl       = ""
	restLuminaUrl = ""
	// Amount in USD that is used to simulate trades.
	amountInUSDConstant = float64(100)
	// fees are ints with precision 6.
	// allFeesV4 = []*big.Int{big.NewInt(100), big.NewInt(500), big.NewInt(3000), big.NewInt(10000)}

	// TO DO: Put the following variables to environment variables.
	DIA_Meta_Contract_Address_V4   = "0x0087342f5f4c7AB23a37c045c3EF710749527c88"
	DIA_Meta_Contract_Precision_V4 = 8
	priceMap_Update_Seconds_V4     = 30 * 60
	simulation_Update_Seconds_V4   = 30
	liquidity_Threshold_USD_V4     = float64(50000)
	liquidity_Threshold_Native_V4  = float64(2)
)

func init() {
	var err error
	// Import and cast environment variables.
	DIA_Meta_Contract_Address_V4 = utils.Getenv("DIA_META_CONTRACT_ADDRESS_V4", DIA_Meta_Contract_Address_V4)
	DIA_Meta_Contract_Precision_V4, err = strconv.Atoi(utils.Getenv("DIA_META_CONTRACT_PRECISION_V4", strconv.Itoa(DIA_Meta_Contract_Precision_V4)))
	if err != nil {
		log.Errorf("DIA_META_CONTRACT_PRECISION: %v", err)
	}
	priceMap_Update_Seconds_V4, err = strconv.Atoi(utils.Getenv(strings.ToUpper(UNISWAPV4_SIMULATION)+"_PRICE_MAP_UPDATE_SECONDS", strconv.Itoa(priceMap_Update_Seconds_V4)))
	if err != nil {
		log.Errorf(strings.ToUpper(UNISWAPV4_SIMULATION)+"_PRICE_MAP_UPDATE_SECONDS: %v", err)
	}
	simulation_Update_Seconds_V4, err = strconv.Atoi(utils.Getenv(strings.ToUpper(UNISWAPV4_SIMULATION)+"_SIMULATION_UPDATE_SECONDS", strconv.Itoa(simulation_Update_Seconds_V4)))
	if err != nil {
		log.Errorf(strings.ToUpper(UNISWAPV4_SIMULATION)+"_SIMULATION_UPDATE_SECONDS: %v", err)
	}
	liquidity_Threshold_USD_V4, err = strconv.ParseFloat(utils.Getenv(strings.ToUpper(UNISWAPV4_SIMULATION)+"_LIQUIDITY_THRESHOLD_USD", strconv.Itoa(int(liquidity_Threshold_USD_V4))), 64)
	if err != nil {
		log.Errorf(strings.ToUpper(UNISWAPV4_SIMULATION)+"_LIQUIDITY_THRESHOLD_USD: %v", err)
	}
	liquidity_Threshold_Native_V4, err = strconv.ParseFloat(utils.Getenv(strings.ToUpper(UNISWAPV4_SIMULATION)+"_LIQUIDITY_THRESHOLD_NATIVE", strconv.Itoa(int(liquidity_Threshold_Native_V4))), 64)
	if err != nil {
		log.Errorf(strings.ToUpper(UNISWAPV4_SIMULATION)+"_LIQUIDITY_THRESHOLD_NATIVE: %v", err)
	}
}

func NewUniswapV4Simulator(exchangepairs []models.ExchangePair, tradesChannel chan models.SimulatedTrade) {
	s := &UniswapV4Simulator{
		exchangepairs: exchangepairs,
		priceMap:      make(map[models.Asset]models.AssetQuotation),
	}

	var err error
	s.restClient, err = ethclient.Dial(utils.Getenv(strings.ToUpper(UNISWAPV4_SIMULATION)+"_URI_REST", restUrl))
	if err != nil {
		log.Fatal("Failed to connect to ETH node: ", err)
	} else {
		log.Info("Successfully connected to node")
	}
	defer s.restClient.Close()

	s.luminaClient, err = ethclient.Dial(utils.Getenv(strings.ToUpper(UNISWAPV4_SIMULATION)+"_LUMINA_URI_REST", restLuminaUrl))
	if err != nil {
		log.Error("init lumina client: ", err)
	} else {
		log.Info("Successfully connected to lumina node")
	}
	defer s.luminaClient.Close()

	quoterAddr := common.HexToAddress(utils.Getenv(strings.ToUpper(UNISWAPV4_SIMULATION)+"_QUOTER", "0x000000000004444c5dc75cB358380D2e3dE08A90"))
	s.quoter, err = v4quoter.NewV4Quoter(quoterAddr, s.restClient)
	if err != nil {
		log.Fatal("Failed to instantiate V4Quoter: ", err)
	}

	s.poolManager, err = poolManager.NewPoolManager(common.HexToAddress(utils.Getenv(strings.ToUpper(UNISWAPV4_SIMULATION)+"_POOLMANAGER", "0x000000000004444c5dc75cB358380D2e3dE08A90")), s.restClient)
	if err != nil {
		log.Fatal("Failed to instantiate PoolManager: ", err)
	}

	err = s.getExchangePairs()
	if err != nil {
		log.Fatal("Failed to get exchange pairs: ", err)
	}

	var lock sync.RWMutex
	s.updatePriceMap(&lock)

	priceTicker := time.NewTicker(time.Duration(priceMapUpdateSeconds) * time.Second)
	go func() {
		for range priceTicker.C {
			s.updatePriceMap(&lock)
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		s.simulateTrades(tradesChannel)
	}
}

func (s *UniswapV4Simulator) getExchangePairs() error {
	for i, ep := range s.exchangepairs {
		quote, err := models.GetAsset(common.HexToAddress(ep.UnderlyingPair.QuoteToken.Address), Exchanges[UNISWAPV4_SIMULATION].Blockchain, s.restClient)
		if err != nil {
			return err
		}
		base, err := models.GetAsset(common.HexToAddress(ep.UnderlyingPair.BaseToken.Address), Exchanges[UNISWAPV4_SIMULATION].Blockchain, s.restClient)
		if err != nil {
			return err
		}
		s.exchangepairs[i].UnderlyingPair.QuoteToken = quote
		s.exchangepairs[i].UnderlyingPair.BaseToken = base
		s.priceMap[quote] = models.AssetQuotation{}
		s.priceMap[base] = models.AssetQuotation{}
	}
	return nil
}

func (scraper *UniswapV4Simulator) updatePriceMap(lock *sync.RWMutex) {
	for asset := range scraper.priceMap {
		quotation, err := asset.GetOnchainPrice(common.HexToAddress(DIAMetaContractAddress), DIAMetaContractPrecision, scraper.luminaClient)
		if err != nil {
			log.Errorf("GetOnchainPrice for %s -- %s: %v", asset.Symbol, asset.Address, err)
			quotation.Price = scraper.getPriceFromAPI(asset)
		} else {
			log.Infof("USD price for (base-)token %s: %v", asset.Symbol, quotation.Price)
		}
		if quotation.Price == 0 {
			quotation.Price = scraper.getPriceFromAPI(asset)
		}
		lock.Lock()
		scraper.priceMap[asset] = quotation
		lock.Unlock()
	}
}

func (scraper *UniswapV4Simulator) getPriceFromAPI(asset models.Asset) float64 {
	log.Warnf("Could not determine price of %s on chain. Checking DIA API.", asset.Symbol)
	price, err := utils.GetPriceFromDiaAPI(asset.Address, asset.Blockchain)
	if err != nil {
		log.Errorf("Failed to get price of %s from DIA API: %v\n", asset.Symbol, err)
		log.Errorf("asset blockchain: %v\n", asset.Blockchain)
		log.Errorf("asset address: %v\n", asset.Address)
		price = 100
	} else {
		log.Infof("Fetched price of %s from DIA API: %.4f", asset.Symbol, price)
	}
	return price
}

func FloatToBigInt(amount float64, decimals uint8) *big.Int {
	f := new(big.Float).Mul(big.NewFloat(amount), big.NewFloat(0).SetFloat64(float64Pow(10, int(decimals))))
	result := new(big.Int)
	f.Int(result)
	return result
}

func float64Pow(a, b int) float64 {
	return float64(new(big.Int).Exp(big.NewInt(int64(a)), big.NewInt(int64(b)), nil).Int64())
}

func (s *UniswapV4Simulator) simulateTrades(tradesChannel chan models.SimulatedTrade) {
	var wg sync.WaitGroup

	for _, ep := range s.exchangepairs {
		wg.Add(1)
		go func(ep models.ExchangePair) {
			defer wg.Done()

			basePrice := s.priceMap[ep.UnderlyingPair.BaseToken].Price
			amountInBase := amountInUSDConstant / basePrice

			decimals := ep.UnderlyingPair.BaseToken.Decimals
			amountIn := FloatToBigInt(amountInBase, decimals)
			// amountIn := big.NewInt(int64(amountInBase))
			log.Infof("[UniswapV4 Simulate] amountIn & amountInBase: %s | %v", amountIn.String(), amountInBase)

			poolKey := v4quoter.PoolKey{
				Currency0:   common.HexToAddress(ep.UnderlyingPair.QuoteToken.Address), // WBTC
				Currency1:   common.HexToAddress(ep.UnderlyingPair.BaseToken.Address),  // USDT
				Fee:         big.NewInt(3000),                                          // 0.3%
				TickSpacing: big.NewInt(60),                                            // default for 0.3% tier
				Hooks:       common.HexToAddress("0x0000000000000000000000000000000000000000"),
			}

			params := v4quoter.IV4QuoterQuoteExactSingleParams{
				PoolKey:     poolKey,
				ZeroForOne:  true,     // or false，depends on the direction
				ExactAmount: amountIn, // *big.Int
				HookData:    []byte{},
			}

			log.Infof(
				"[UniswapV4 Simulate] PoolKey: {Token0: %s, Token1: %s, Fee: %d, TickSpacing: %s, Hooks: %s} | ZeroForOne: %v | ExactAmount: %s | HookData: %x",
				params.PoolKey.Currency0.Hex(),
				params.PoolKey.Currency1.Hex(),
				params.PoolKey.Fee,
				params.PoolKey.TickSpacing.String(),
				params.PoolKey.Hooks.Hex(),
				params.ZeroForOne,
				params.ExactAmount.String(),
				params.HookData,
			)

			amountOut, err := s.quoter.QuoteExactInputSingle(&bind.CallOpts{Context: context.Background()}, params)
			if err != nil {
				log.Warnf("QuoteExactInputSingle failed: %v", err)
				return
			}

			price := new(big.Float).Quo(new(big.Float).SetFloat64(amountInUSDConstant), new(big.Float).SetInt(amountOut.AmountOut))
			priceF, _ := price.Float64()
			amountOutF := WeiToFloat(amountOut.AmountOut, ep.UnderlyingPair.QuoteToken.Decimals)

			trade := models.SimulatedTrade{
				Price:       priceF,
				Volume:      amountOutF,
				QuoteToken:  ep.UnderlyingPair.QuoteToken,
				BaseToken:   ep.UnderlyingPair.BaseToken,
				PoolAddress: "0x000000000004444c5dc75cB358380D2e3dE08A90",
				Time:        time.Now(),
				Exchange:    Exchanges[UNISWAPV4_SIMULATION],
			}
			tradesChannel <- trade
			log.Infof(
				"[UniswapV4 Simulate] amountIn: %s | amountOut: %s | price: %.6f | volume: %.6f | base: %s | quote: %s",
				amountIn.String(),
				amountOut.AmountOut.String(),
				trade.Price,
				trade.Volume,
				trade.BaseToken.Symbol,
				trade.QuoteToken.Symbol,
			)
		}(ep)
	}
	wg.Wait()
}

func WeiToFloat(amount *big.Int, decimals uint8) float64 {
	amt := new(big.Float).SetInt(amount)
	dec := new(big.Float).SetFloat64(float64Pow(10, int(decimals)))
	val, _ := new(big.Float).Quo(amt, dec).Float64()
	return val
}
