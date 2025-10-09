package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/diadata-org/lumina-library/utils"
	"github.com/tkanos/gonfig"
)

// Pool is the container for liquidity pools on DEXes.
type Pool struct {
	Exchange     Exchange      `json:"Exchange"`
	Blockchain   Blockchain    `json:"Blockchain"`
	Address      string        `json:"Address"`
	Assetvolumes []AssetVolume `json:"Assetvolumes"`
	Order        int           `json:"Order"`
	Time         time.Time     `json:"Time"`
}

type AssetVolume struct {
	Asset  Asset   `json:"Asset"`
	Volume float64 `json:"Volume"`
	Index  uint8   `json:"Index"`
}

// MakePoolMap maps an exchange name on the underlying slice of pool structs.
func MakePoolMap(pools []Pool) map[string][]Pool {
	poolMap := make(map[string][]Pool)
	for _, pool := range pools {
		poolMap[pool.Exchange.Name] = append(poolMap[pool.Exchange.Name], pool)
	}
	return poolMap
}

func PoolsFromEnv(poolsEnv string, envSeparator string, exchangePoolSeparator string) (pools []Pool, err error) {
	list := strings.Split(poolsEnv, envSeparator)
	if len(list) == 0 {
		return
	}
	if len(list) == 1 && len(strings.TrimSpace(list[0])) == 0 {
		return
	}

	for _, ep := range strings.Split(poolsEnv, envSeparator) {
		poolInfo := strings.Split(ep, exchangePoolSeparator)
		if len(poolInfo) != 3 {
			err = fmt.Errorf("pool info does not have length 3: %s", ep)
			return
		}
		var p Pool
		p.Exchange.Name = strings.TrimSpace(poolInfo[0])
		p.Address = strings.TrimSpace(poolInfo[1])
		p.Order, err = strconv.Atoi(strings.TrimSpace(poolInfo[2]))
		if err != nil {
			return
		}
		pools = append(pools, p)
	}
	return
}

func GetPoolsFromConfig(exchange string) ([]Pool, error) {
	path := utils.GetPath("pools/", exchange)
	type Pools struct {
		Pools []Pool
	}
	var p Pools
	err := gonfig.GetConf(path, &p)
	if err != nil {
		return []Pool{}, err
	}
	return p.Pools, nil
}
