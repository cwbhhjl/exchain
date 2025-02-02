package evm

import (
	"github.com/okex/exchain/x/evm/keeper"
	"github.com/okex/exchain/x/evm/types"
)

// nolint
const (
	ModuleName        = types.ModuleName
	StoreKey          = types.StoreKey
	RouterKey         = types.RouterKey
	DefaultParamspace = types.DefaultParamspace
)

// nolint
var (
	NewKeeper         = keeper.NewKeeper
	TxDecoder         = types.TxDecoder
	NewSimulateKeeper = keeper.NewSimulateKeeper
)

//nolint
type (
	Keeper       = keeper.Keeper
	GenesisState = types.GenesisState
)
