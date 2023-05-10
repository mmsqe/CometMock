package rpc_server

import (
	"fmt"

	"github.com/cometbft/cometbft/libs/bytes"
	cmtmath "github.com/cometbft/cometbft/libs/math"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	rpc "github.com/cometbft/cometbft/rpc/jsonrpc/server"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/cometbft/cometbft/types"
	"github.com/p-offtermatt/CometMock/cometmock/abci_client"
)

const (
	defaultPerPage = 30
	maxPerPage     = 100
)

var Routes = map[string]*rpc.RPCFunc{
	// info API
	"validators": rpc.NewRPCFunc(Validators, "height,page,per_page"),
	"block":      rpc.NewRPCFunc(Block, "height", rpc.Cacheable("height")),

	// // tx broadcast API
	"broadcast_tx_commit": rpc.NewRPCFunc(BroadcastTxCommit, "tx"),
	"broadcast_tx_sync":   rpc.NewRPCFunc(BroadcastTxSync, "tx"),
	"broadcast_tx_async":  rpc.NewRPCFunc(BroadcastTxAsync, "tx"),

	// // abci API
	"abci_query": rpc.NewRPCFunc(ABCIQuery, "path,data,height,prove"),
}

// BroadcastTxCommit broadcasts a transaction,
// and wait until it is included in a block and and comitted.
// In our case, this means running a block with just the the transition,
// then return.
func BroadcastTxCommit(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	abci_client.GlobalClient.Logger.Info(
		"BroadcastTxCommut called", "tx", tx)

	return BroadcastTx(&tx)
}

// BroadcastTxSync would normally broadcast a transaction and wait until it gets the result from CheckTx.
// In our case, we run a block with just the transition in it,
// then return.
func BroadcastTxSync(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	abci_client.GlobalClient.Logger.Info(
		"BroadcastTxSync called", "tx", tx)

	_, err := BroadcastTx(&tx)
	if err != nil {
		return nil, err
	}

	return &ctypes.ResultBroadcastTx{}, nil
}

// BroadcastTxAsync would normally broadcast a transaction and return immediately.
// In our case, we always include the transition in the next block, and return when that block is committed.
// ResultBroadcastTx is empty, since we do not return the result of CheckTx nor DeliverTx.
func BroadcastTxAsync(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	abci_client.GlobalClient.Logger.Info(
		"BroadcastTxAsync called", "tx", tx)

	_, err := BroadcastTx(&tx)
	if err != nil {
		return nil, err
	}

	return &ctypes.ResultBroadcastTx{}, nil
}

// BroadcastTx delivers a transaction to the ABCI client, includes it in the next block, then returns.
func BroadcastTx(tx *types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	abci_client.GlobalClient.Logger.Info(
		"BroadcastTxs called", "tx", tx)

	byteTx := []byte(*tx)

	_, _, _, _, err := abci_client.GlobalClient.RunBlock(&byteTx)
	if err != nil {
		return nil, err
	}

	// TODO: fill the return value if necessary
	return &ctypes.ResultBroadcastTxCommit{}, nil
}

func ABCIQuery(
	ctx *rpctypes.Context,
	path string,
	data bytes.HexBytes,
	height int64,
	prove bool,
) (*ctypes.ResultABCIQuery, error) {
	abci_client.GlobalClient.Logger.Info(
		"ABCIQuery called", "path", "data", "height", "prove", path, data, height, prove)

	response, err := abci_client.GlobalClient.SendAbciQuery(data, path, height, prove)
	return &ctypes.ResultABCIQuery{Response: *response}, err
}

func Validators(ctx *rpctypes.Context, heightPtr *int64, pagePtr, perPagePtr *int) (*ctypes.ResultValidators, error) {
	// only the last height is available, since we do not keep past heights at the moment
	if heightPtr != nil {
		return nil, fmt.Errorf("height parameter is not supported, use version of the function without height")
	}

	height := abci_client.GlobalClient.CurState.LastBlockHeight

	validators := abci_client.GlobalClient.CurState.LastValidators

	totalCount := len(validators.Validators)
	perPage := validatePerPage(perPagePtr)
	page, err := validatePage(pagePtr, perPage, totalCount)
	if err != nil {
		return nil, err
	}

	skipCount := validateSkipCount(page, perPage)

	v := validators.Validators[skipCount : skipCount+cmtmath.MinInt(perPage, totalCount-skipCount)]

	return &ctypes.ResultValidators{
		BlockHeight: height,
		Validators:  v,
		Count:       len(v),
		Total:       totalCount,
	}, nil
}

// validatePage is adapted from https://github.com/cometbft/cometbft/blob/9267594e0a17c01cc4a97b399ada5eaa8a734db5/rpc/core/env.go#L107
func validatePage(pagePtr *int, perPage, totalCount int) (int, error) {
	if perPage < 1 {
		panic(fmt.Sprintf("zero or negative perPage: %d", perPage))
	}

	if pagePtr == nil { // no page parameter
		return 1, nil
	}

	pages := ((totalCount - 1) / perPage) + 1
	if pages == 0 {
		pages = 1 // one page (even if it's empty)
	}
	page := *pagePtr
	if page <= 0 || page > pages {
		return 1, fmt.Errorf("page should be within [1, %d] range, given %d", pages, page)
	}

	return page, nil
}

// validatePerPage is adapted from https://github.com/cometbft/cometbft/blob/9267594e0a17c01cc4a97b399ada5eaa8a734db5/rpc/core/env.go#L128
func validatePerPage(perPagePtr *int) int {
	if perPagePtr == nil { // no per_page parameter
		return defaultPerPage
	}

	perPage := *perPagePtr
	if perPage < 1 {
		return defaultPerPage
	} else if perPage > maxPerPage {
		return maxPerPage
	}
	return perPage
}

// validateSkipCount is adapted from https://github.com/cometbft/cometbft/blob/9267594e0a17c01cc4a97b399ada5eaa8a734db5/rpc/core/env.go#L171
func validateSkipCount(page, perPage int) int {
	skipCount := (page - 1) * perPage
	if skipCount < 0 {
		return 0
	}

	return skipCount
}

func Block(ctx *rpctypes.Context, heightPtr *int64) (*ctypes.ResultBlock, error) {
	// only the last height is available, since we do not keep past heights at the moment
	if heightPtr != nil {
		return nil, fmt.Errorf("height parameter is not supported, use version of the function without height")
	}

	blockID := abci_client.GlobalClient.CurState.LastBlockID

	// TODO: return an actual block if it is needed, for now return en empty block
	block := &types.Block{Header: types.Header{Height: abci_client.GlobalClient.CurState.LastBlockHeight}}

	return &ctypes.ResultBlock{BlockID: blockID, Block: block}, nil
}
