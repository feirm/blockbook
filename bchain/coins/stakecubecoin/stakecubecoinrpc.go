package stakecubecoin

import (
	"encoding/json"
	"math/big"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/trezor/blockbook/bchain"
	"github.com/trezor/blockbook/bchain/coins/btc"
)

// StakeCubeCoinRPC is an interface to JSON-RPC bitcoind service.
type StakeCubeCoinRPC struct {
	*btc.BitcoinRPC
	minFeeRate *big.Int // satoshi per kb
}

// NewStakeCubeCoinRPC returns new StakeCubeCoinRPC instance.
func NewStakeCubeCoinRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	b, err := btc.NewBitcoinRPC(config, pushHandler)
	if err != nil {
		return nil, err
	}

	s := &StakeCubeCoinRPC{
		b.(*btc.BitcoinRPC),
		big.NewInt(4000000),
	}
	s.RPCMarshaler = btc.JSONMarshalerV1{}
	s.ChainConfig.SupportsEstimateSmartFee = true

	return s, nil
}

// Initialize initializes StakeCubeCoinRPC instance.
func (b *StakeCubeCoinRPC) Initialize() error {
	ci, err := b.GetChainInfo()
	if err != nil {
		return err
	}
	chainName := ci.Chain

	params := GetChainParams(chainName)

	// always create parser
	b.Parser = NewStakeCubeCoinParser(params, b.ChainConfig)

	// parameters for getInfo request
	if params.Net == MainnetMagic {
		b.Testnet = false
		b.Network = "livenet"
	} else {
		b.Testnet = true
		b.Network = "testnet"
	}

	glog.Info("rpc: block chain ", params.Name)

	return nil
}

// GetBlock returns block with given hash.
func (s *StakeCubeCoinRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
	var err error
	if hash == "" && height > 0 {
		hash, err = s.GetBlockHash(height)
		if err != nil {
			return nil, err
		}
	}

	glog.V(1).Info("rpc: getblock (verbosity=1) ", hash)

	res := btc.ResGetBlockThin{}
	req := btc.CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbosity = 1
	err = s.Call(&req, &res)

	if err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}
	if res.Error != nil {
		return nil, errors.Annotatef(res.Error, "hash %v", hash)
	}

	txs := make([]bchain.Tx, 0, len(res.Result.Txids))
	for _, txid := range res.Result.Txids {
		tx, err := s.GetTransaction(txid)
		if err != nil {
			if err == bchain.ErrTxNotFound {
				glog.Errorf("rpc: getblock: skipping transanction in block %s due error: %s", hash, err)
				continue
			}
			return nil, err
		}
		txs = append(txs, *tx)
	}
	block := &bchain.Block{
		BlockHeader: res.Result.BlockHeader,
		Txs:         txs,
	}
	return block, nil
}

// GetTransactionForMempool returns a transaction by the transaction ID.
// It could be optimized for mempool, i.e. without block time and confirmations
func (s *StakeCubeCoinRPC) GetTransactionForMempool(txid string) (*bchain.Tx, error) {
	return s.GetTransaction(txid)
}

// EstimateSmartFee returns fee estimation
func (s *StakeCubeCoinRPC) EstimateSmartFee(blocks int, conservative bool) (big.Int, error) {
	feeRate, err := s.BitcoinRPC.EstimateSmartFee(blocks, conservative)
	if err != nil {
		if s.minFeeRate.Cmp(&feeRate) == 1 {
			feeRate = *s.minFeeRate
		}
	}
	return feeRate, err
}
