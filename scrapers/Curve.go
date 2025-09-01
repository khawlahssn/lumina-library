package scrapers

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/diadata-org/lumina-library/contracts/curve/curvefactory"
	"github.com/diadata-org/lumina-library/contracts/curve/curvefi"
	"github.com/diadata-org/lumina-library/contracts/curve/curvefifactory"
	curvefitwocryptooptimized "github.com/diadata-org/lumina-library/contracts/curve/curvefitwocrypto"
	"github.com/diadata-org/lumina-library/contracts/curve/curvepool"
	models "github.com/diadata-org/lumina-library/models"
	"github.com/diadata-org/lumina-library/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
)

var (
	restDialCurve     = ""
	wsDialCurve       = ""
	registryAddresses = map[string]string{
		"base":       "0x90E00ACe148ca3b23Ac1bC8C240C2a7Dd9c2d7f5",
		"factory":    "0xF18056Bbd320E96A48e3Fbf8bC061322531aac99",
		"cryptoswap": "0x8F942C20D02bEfc377D41445793068908E2250D0",
		"meta":       "0xB9fC157394Af804a3578134A6585C0dc9cc990d4",
		"factory2":   "0x4F8846Ae9380B90d2E71D5e3D042dff3E7ebb40d",
	}
)

type CurvePair struct {
	OutAsset    models.Asset
	InAsset     models.Asset
	OutIndex    int
	InIndex     int
	ForeignName string
	Address     common.Address
}

type CurveSwap struct {
	ID        string
	Timestamp int64
	Pair      CurvePair
	Amount0   float64
	Amount1   float64
}

type CurveScraper struct {
	exchange         models.Exchange
	poolMap          map[common.Address]CurvePair
	subscribeChannel chan common.Address
	lastTradeTimeMap map[common.Address]time.Time
	restClient       *ethclient.Client
	wsClient         *ethclient.Client
	waitTime         int
}

func NewCurveScraper(ctx context.Context, exchangeName string, blockchain string, pools []models.Pool, tradesChannel chan models.Trade, wg *sync.WaitGroup) {
	var err error
	var scraper CurveScraper
	log.Info("Started Curve scraper.")

	scraper.exchange.Name = exchangeName
	scraper.exchange.Blockchain = blockchain
	scraper.subscribeChannel = make(chan common.Address)
	scraper.lastTradeTimeMap = make(map[common.Address]time.Time)
	scraper.waitTime, err = strconv.Atoi(utils.Getenv(strings.ToUpper(exchangeName)+"_WAIT_TIME", "500"))
	if err != nil {
		log.Error("parse waitTime: ", err)
	}

	scraper.restClient, err = ethclient.Dial(utils.Getenv(CURVE_EXCHANGE+"_URI_REST", restDialCurve))
	if err != nil {
		log.Error("Curve - init rest client: ", err)
	}
	scraper.wsClient, err = ethclient.Dial(utils.Getenv(CURVE_EXCHANGE+"_URI_WS", wsDialCurve))
	if err != nil {
		log.Error("Curve - init ws client: ", err)
	}

	scraper.makePoolMap(pools)

	var lock sync.RWMutex
	go scraper.mainLoop(ctx, pools, tradesChannel, &lock)
}

func (scraper *CurveScraper) mainLoop(ctx context.Context, pools []models.Pool, tradesChannel chan models.Trade, lock *sync.RWMutex) {
	var wg sync.WaitGroup
	for _, pool := range pools {
		lock.Lock()
		scraper.lastTradeTimeMap[common.HexToAddress(pool.Address)] = time.Now()
		lock.Unlock()

		envVar := strings.ToUpper(scraper.exchange.Name) + "_WATCHDOG_" + pool.Address
		watchdogDelay, err := strconv.ParseInt(utils.Getenv(envVar, "60"), 10, 64)
		if err != nil {
			log.Errorf("Curve - Parse curveWatchdogDelay: %v.", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
		go watchdogPool(ctx, scraper.exchange.Name, common.HexToAddress(pool.Address), watchdogTicker, scraper.lastTradeTimeMap, watchdogDelay, scraper.subscribeChannel, lock)

		time.Sleep(time.Duration(scraper.waitTime) * time.Millisecond)
		wg.Add(1)
		go func(ctx context.Context, address common.Address, w *sync.WaitGroup, lock *sync.RWMutex) {
			defer w.Done()
			scraper.watchSwaps(ctx, address, tradesChannel, lock)
		}(ctx, common.HexToAddress(pool.Address), &wg, lock)
	}
	go func() {
		for {
			select {
			case addr := <-scraper.subscribeChannel:
				log.Infof("Resubscribing to pool: %s", addr.Hex())

				lock.Lock()
				scraper.lastTradeTimeMap[addr] = time.Now()
				lock.Unlock()

				go scraper.watchSwaps(ctx, addr, tradesChannel, lock)

			case <-ctx.Done():
				log.Info("Stopping resubscription handler.")
				return
			}
		}
	}()
	wg.Wait()
}

func getSwapDataCurve(swap CurveSwap) (price float64, volume float64) {
	volume = swap.Amount1
	price = swap.Amount0 / swap.Amount1
	return
}

func makeCurveTrade(
	pair CurvePair,
	price float64,
	volume float64,
	timestamp time.Time,
	address common.Address,
	foreignTradeID string,
	exchangeName string,
	blockchain string,
) models.Trade {
	token0 := pair.OutAsset
	token1 := pair.InAsset
	return models.Trade{
		Price:          price,
		Volume:         volume,
		BaseToken:      token1,
		QuoteToken:     token0,
		Time:           timestamp,
		PoolAddress:    address.Hex(),
		ForeignTradeID: foreignTradeID,
		Exchange:       models.Exchange{Name: exchangeName, Blockchain: blockchain},
	}
}

func parseIndexCode(code int) (outIdx, inIdx int, err error) {
	s := strconv.Itoa(code)
	if len(s) != 2 {
		return 0, 0, fmt.Errorf("index code must be 2 digits, got: %s", s)
	}

	outIdx = int(s[0] - '0')
	inIdx = int(s[1] - '0')
	if outIdx < 0 || inIdx < 0 || outIdx == inIdx {
		return 0, 0, fmt.Errorf("invalid index code: %s", s)
	}
	return
}

func (scraper *CurveScraper) watchSwaps(ctx context.Context, address common.Address, tradesChannel chan models.Trade, lock *sync.RWMutex) {
	pair := scraper.poolMap[address]

	feeds, err := scraper.GetSwapsChannel(address)
	if err != nil {
		log.Error("Curve - failed to get swaps channel: ", err)
		return
	}
	log.Infof("Curve - subscribed to pool: %s", address.Hex())

	go func() {
		for {
			select {
			case rawSwap, ok := <-feeds.Sink:
				if ok {
					log.Infof("Curve - received swap: %v", rawSwap)
					swap, err := scraper.normalizeCurveSwap(*rawSwap)
					if err != nil {
						log.Error("Curve - error normalizing swap: ", err)
					}
					scraper.processSwap(swap, pair, address, lock, tradesChannel)
				}
			case rawSwapCurvefiFactory, ok := <-feeds.factorySink:
				if ok {
					log.Infof("Curve - received swap: %v", rawSwapCurvefiFactory)
					swap, err := scraper.normalizeCurveSwap(*rawSwapCurvefiFactory)
					if err != nil {
						log.Error("Curve - error normalizing swap: ", err)
					}
					scraper.processSwap(swap, pair, address, lock, tradesChannel)
				}
			case rawSwapTwoCrypto, ok := <-feeds.twoSink:
				if ok {
					log.Infof("Curve - received swap: %v", rawSwapTwoCrypto)
					swap, err := scraper.normalizeCurveSwap(*rawSwapTwoCrypto)
					if err != nil {
						log.Error("Curve - error normalizing swap: ", err)
					}
					scraper.processSwap(swap, pair, address, lock, tradesChannel)
				}
			case rawSwapUnderlying, ok := <-feeds.underlyingSink:
				if ok {
					log.Infof("Curve - received swap: %v", rawSwapUnderlying)
					swap, err := scraper.normalizeCurveSwap(*rawSwapUnderlying)
					if err != nil {
						log.Error("Curve - error normalizing swap: ", err)
					}
					scraper.processSwap(swap, pair, address, lock, tradesChannel)
				}
			case err := <-feeds.Sub.Err():
				log.Errorf("Subscription error for pool %s: %v", address.Hex(), err)
				scraper.subscribeChannel <- address
				return
			case err := <-feeds.factorySub.Err():
				log.Errorf("Subscription error for pool %s: %v", address.Hex(), err)
				scraper.subscribeChannel <- address
				return
			case err := <-feeds.twoSub.Err():
				log.Errorf("Subscription error for pool %s: %v", address.Hex(), err)
				scraper.subscribeChannel <- address
				return
			case err := <-feeds.underlyingSub.Err():
				log.Errorf("Subscription error for pool %s: %v", address.Hex(), err)
				scraper.subscribeChannel <- address
				return
			case <-ctx.Done():
				log.Infof("Sutting down watchSwaps for %s", address.Hex())
				return
			}
		}
	}()
}

func (scraper *CurveScraper) processSwap(swap CurveSwap, pair CurvePair, address common.Address, lock *sync.RWMutex, tradesChannel chan models.Trade) {
	price, volume := getSwapDataCurve(swap)
	t := makeCurveTrade(pair, price, volume, time.Unix(swap.Timestamp, 0), address, swap.ID, scraper.exchange.Name, scraper.exchange.Blockchain)

	// Update lastTradeTimeMap
	lock.Lock()
	scraper.lastTradeTimeMap[address] = t.Time
	lock.Unlock()

	tradesChannel <- t
	logTrade(t)
}

func (scraper *CurveScraper) normalizeCurveSwap(swap interface{}) (CurveSwap, error) {
	var pair CurvePair
	var amount0 float64
	var amount1 float64
	var swapId string

	switch s := swap.(type) {
	case curvepool.CurvepoolTokenExchange:
		pair := scraper.poolMap[s.Raw.Address]
		decimals0 := int(pair.OutAsset.Decimals)
		decimals1 := int(pair.InAsset.Decimals)
		swapId = s.Raw.TxHash.Hex()

		amount0, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(s.TokensSold), new(big.Float).SetFloat64(math.Pow10(decimals0))).Float64()
		amount1, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(s.TokensBought), new(big.Float).SetFloat64(math.Pow10(decimals1))).Float64()

	case curvefifactory.CurvefifactoryTokenExchange:
		pair := scraper.poolMap[s.Raw.Address]
		decimals0 := int(pair.OutAsset.Decimals)
		decimals1 := int(pair.InAsset.Decimals)
		swapId = s.Raw.TxHash.Hex()

		amount0, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(s.TokensSold), new(big.Float).SetFloat64(math.Pow10(decimals0))).Float64()
		amount1, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(s.TokensBought), new(big.Float).SetFloat64(math.Pow10(decimals1))).Float64()

	case curvefitwocryptooptimized.CurvefitwocryptooptimizedTokenExchange:
		pair := scraper.poolMap[s.Raw.Address]
		decimals0 := int(pair.OutAsset.Decimals)
		decimals1 := int(pair.InAsset.Decimals)
		swapId = s.Raw.TxHash.Hex()

		amount0, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(s.TokensSold), new(big.Float).SetFloat64(math.Pow10(decimals0))).Float64()
		amount1, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(s.TokensBought), new(big.Float).SetFloat64(math.Pow10(decimals1))).Float64()

	case curvepool.CurvepoolTokenExchangeUnderlying:
		pair := scraper.poolMap[s.Raw.Address]
		decimals0 := int(pair.OutAsset.Decimals)
		decimals1 := int(pair.InAsset.Decimals)
		swapId = s.Raw.TxHash.Hex()

		amount0, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(s.TokensSold), new(big.Float).SetFloat64(math.Pow10(decimals0))).Float64()
		amount1, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(s.TokensBought), new(big.Float).SetFloat64(math.Pow10(decimals1))).Float64()

	}
	normalizedSwap := CurveSwap{
		ID:        swapId,
		Timestamp: time.Now().Unix(),
		Pair:      pair,
		Amount0:   amount0,
		Amount1:   amount1,
	}

	return normalizedSwap, nil
}

type curveFeeds struct {
	Sink           chan *curvepool.CurvepoolTokenExchange
	Sub            event.Subscription
	factorySink    chan *curvefifactory.CurvefifactoryTokenExchange
	factorySub     event.Subscription
	twoSink        chan *curvefitwocryptooptimized.CurvefitwocryptooptimizedTokenExchange
	twoSub         event.Subscription
	underlyingSink chan *curvepool.CurvepoolTokenExchangeUnderlying
	underlyingSub  event.Subscription
}

func (scraper *CurveScraper) GetSwapsChannel(address common.Address) (*curveFeeds, error) {
	filterer, err := curvepool.NewCurvepoolFilterer(address, scraper.wsClient)
	if err != nil {
		log.Fatal(err)
	}

	curvefiFactoryFilterer, err := curvefifactory.NewCurvefifactoryFilterer(address, scraper.wsClient)
	if err != nil {
		log.Fatal(err)
	}

	filtererTwoCrypto, err := curvefitwocryptooptimized.NewCurvefitwocryptooptimizedFilterer(address, scraper.wsClient)
	if err != nil {
		log.Fatal(err)
	}

	sink := make(chan *curvepool.CurvepoolTokenExchange)
	sinkCurvefiFactory := make(chan *curvefifactory.CurvefifactoryTokenExchange)
	sinkTwoCrypto := make(chan *curvefitwocryptooptimized.CurvefitwocryptooptimizedTokenExchange)
	sinkUnderlying := make(chan *curvepool.CurvepoolTokenExchangeUnderlying)

	header, err := scraper.restClient.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}
	startblock := header.Number.Uint64() - uint64(20)

	feeds := &curveFeeds{
		Sink:           make(chan *curvepool.CurvepoolTokenExchange),
		factorySink:    make(chan *curvefifactory.CurvefifactoryTokenExchange),
		twoSink:        make(chan *curvefitwocryptooptimized.CurvefitwocryptooptimizedTokenExchange),
		underlyingSink: make(chan *curvepool.CurvepoolTokenExchangeUnderlying),
	}

	sub, err := filterer.WatchTokenExchange(&bind.WatchOpts{Start: &startblock}, sink, nil)
	if err != nil {
		log.Error("error in get swaps channel: ", err)
	}
	feeds.Sub = sub

	curvefiFactorySub, err := curvefiFactoryFilterer.WatchTokenExchange(&bind.WatchOpts{Start: &startblock}, sinkCurvefiFactory, nil)
	if err != nil {
		log.Error("error in get swaps channel: ", err)
	}
	feeds.factorySub = curvefiFactorySub

	subTwoCrypto, err := filtererTwoCrypto.WatchTokenExchange(&bind.WatchOpts{Start: &startblock}, sinkTwoCrypto, nil)
	if err != nil {
		log.Error("WatchTokenExchange for twoCrypto: ", err)
	}
	feeds.twoSub = subTwoCrypto

	subUnderlying, err := filterer.WatchTokenExchangeUnderlying(&bind.WatchOpts{Start: &startblock}, sinkUnderlying, nil)
	if err != nil {
		log.Error("WatchTokenExchangeUnderlying: ", err)
	}
	feeds.underlyingSub = subUnderlying

	return feeds, nil
}

func (scraper *CurveScraper) makePoolMap(pools []models.Pool) error {
	scraper.poolMap = make(map[common.Address]CurvePair)
	var assetMap = make(map[common.Address]models.Asset)

	var registry interface{}
	var err error

	registry, err = curvefi.NewCurvefi(common.HexToAddress(registryAddresses["base"]), scraper.restClient)
	if err == nil {
		log.Infof("Curve - created curvefi instance")
	} else {
		registry, err = curvefactory.NewCurvefactory(common.HexToAddress(registryAddresses["factory"]), scraper.restClient)
		if err != nil {
			log.Errorf("Curve - failed to create registry instance: %v", err)
			return err
		} else {
			log.Infof("Curve - created curvefactory instance: factory")
		}
	}

	for _, pool := range pools {
		outIdx, inIdx, err := parseIndexCode(pool.Order)
		if err != nil {
			log.Error("Curve - failed to parse index code: ", err)
			return err
		}
		var coins []common.Address
		switch r := registry.(type) {
		case *curvefi.Curvefi:
			coins_curvefi, err_curvefi := r.GetCoins(&bind.CallOpts{Context: context.Background()}, common.HexToAddress(pool.Address))
			if err_curvefi != nil {
				coins_curvefi, err_curvefi = r.GetUnderlyingCoins(&bind.CallOpts{Context: context.Background()}, common.HexToAddress(pool.Address))
				if err_curvefi != nil {
					log.Errorf("Curve - failed to get coins: %v", err_curvefi)
					return err_curvefi
				}
			}
			coins = coins_curvefi[:]
		case *curvefactory.Curvefactory:
			coins_curvefactory, err_curvefactory := r.GetCoins(&bind.CallOpts{Context: context.Background()}, common.HexToAddress(pool.Address))
			if err_curvefactory != nil {
				log.Errorf("Curve - failed to get coins: %v", err_curvefactory)
				return err_curvefactory
			}
			coins = coins_curvefactory[:]
		default:
			log.Fatal("Curve - unknown registry type")
		}

		if coins[outIdx] == (common.Address{}) || coins[inIdx] == (common.Address{}) {
			log.Errorf("Curve - pool %s - empty coin at index out=%d or in=%d: %v", pool.Address, outIdx, inIdx, err)
			return err
		}

		token0Address := coins[outIdx]
		if _, ok := assetMap[token0Address]; !ok {
			token0, err := models.GetAsset(token0Address, scraper.exchange.Blockchain, scraper.restClient)
			if err != nil {
				return err
			}
			assetMap[token0Address] = token0
		}

		token1Address := coins[inIdx]
		log.Infof("Curve - pool %s - token0Address: %s, token1Address: %s, outIdx: %d, inIdx: %d", pool.Address, token0Address.Hex(), token1Address.Hex(), outIdx, inIdx)
		if _, ok := assetMap[token1Address]; !ok {
			token1, err := models.GetAsset(token1Address, scraper.exchange.Blockchain, scraper.restClient)
			if err != nil {
				return err
			}
			assetMap[token1Address] = token1
		}

		scraper.poolMap[common.HexToAddress(pool.Address)] = CurvePair{
			OutAsset: assetMap[token0Address],
			InAsset:  assetMap[token1Address],
			OutIndex: outIdx,
			InIndex:  inIdx,
			Address:  common.HexToAddress(pool.Address),
		}
	}

	return nil
}
