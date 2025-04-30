package utils

import (
	"strconv"
)

const (
	ETHEREUM = "Ethereum"
	BITCOIN  = "Bitcoin"
)

// GetChainID returns the chain ID from the CHAIN_ID environment variable
// or the provided default value if not set or invalid
func GetChainID(defaultID int64) int64 {
	chainIDStr := Getenv("CHAIN_ID", strconv.FormatInt(defaultID, 10))
	chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
	if err != nil {
		return defaultID
	}
	return chainID
}
