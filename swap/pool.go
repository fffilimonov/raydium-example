package swap

import (
	"context"
	"fmt"
	"log"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"main/models"
)

// GetPool x
func GetPool(clientRPC *rpc.Client, fromToken string, toToken string) (models.PoolConfig, bool) {
	poolDb1 := models.GetPoolConfig(fromToken, toToken)
	if poolDb1.BaseMint == fromToken && poolDb1.QuoteMint == toToken {
		return poolDb1, false
	}

	poolDb2 := models.GetPoolConfig(toToken, fromToken)
	if poolDb2.BaseMint == toToken && poolDb2.QuoteMint == fromToken {
		return poolDb2, true
	}

	pools := []models.PoolConfig{}

	pools1, err := getPools(clientRPC, fromToken, toToken)
	if err != nil {
		log.Printf("GetPool err: %v", err)
	} else {
		pools = append(pools, pools1...)
	}

	pools2, err := getPools(clientRPC, toToken, fromToken)
	if err != nil {
		log.Printf("GetPool err: %v", err)
	} else {
		pools = append(pools, pools2...)
	}

	var res models.PoolConfig
	var baseAmount uint64 = 0
	var quoteAmount uint64 = 0
	reverse := false

	log.Printf("pools: %v", pools)
	if len(pools) > 1 {
		for _, pool := range pools {
			ba, qa, err := getPoolAmounts(clientRPC, pool)
			if err != nil {
				log.Printf("GetPool err: %v", err)
			} else {
				if pool.BaseMint == fromToken && pool.QuoteMint == toToken {
					if ba > baseAmount && qa > quoteAmount {
						baseAmount = ba
						quoteAmount = qa
						res = pool
					}
				}
				if pool.QuoteMint == fromToken && pool.BaseMint == toToken {
					if qa > baseAmount && ba > quoteAmount {
						baseAmount = qa
						quoteAmount = ba
						res = pool
					}
				}
			}
		}
	} else {
		if len(pools) == 1 {
			res = pools[0]
		}
	}

	if res.BaseMint == toToken {
		reverse = true
	}

	log.Printf("pool: %v res: %v", res, reverse)

	if reverse == true {
		if res.BaseMint == toToken && res.QuoteMint == fromToken {
			res.Create()
		}
	} else {
		if res.BaseMint == fromToken && res.QuoteMint == toToken {
			res.Create()
		}
	}

	return res, reverse
}

// GetPool x
func getPoolAmounts(clientRPC *rpc.Client, pool models.PoolConfig) (uint64, uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	res, err := clientRPC.GetMultipleAccounts(
		ctx,
		solana.MustPublicKeyFromBase58(pool.BaseVault),
		solana.MustPublicKeyFromBase58(pool.QuoteVault),
	)

	if err != nil {
		return 0, 0, err
	}

	var baseVault token.Account
	err = bin.NewBinDecoder(res.Value[0].Data.GetBinary()).Decode(&baseVault)
	if err != nil {
		return 0, 0, err
	}

	var quoteVault token.Account
	err = bin.NewBinDecoder(res.Value[1].Data.GetBinary()).Decode(&quoteVault)
	if err != nil {
		return 0, 0, err
	}

	log.Printf("GetPool baseVault.Amount %v %v", pool.BaseMint, baseVault.Amount)
	log.Printf("GetPool quoteVault.Amount %v %v", pool.QuoteMint, quoteVault.Amount)
	return baseVault.Amount, quoteVault.Amount, nil
}

// GetPool x
func getPools(clientRPC *rpc.Client, fromToken string, toToken string) ([]models.PoolConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	pools := []models.PoolConfig{}

	from := solana.MustPublicKeyFromBase58(fromToken)
	to := solana.MustPublicKeyFromBase58(toToken)

	resp, err := clientRPC.GetProgramAccountsWithOpts(
		ctx,
		solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8"),
		&rpc.GetProgramAccountsOpts{
			Filters: []rpc.RPCFilter{
				{
					Memcmp: &rpc.RPCFilterMemcmp{
						Offset: 400,
						Bytes:  from[:],
					},
				},
				{
					Memcmp: &rpc.RPCFilterMemcmp{
						Offset: 432,
						Bytes:  to[:],
					},
				},
			},
		},
	)
	if err != nil {
		return pools, err
	}

	for _, res := range resp {
		info := &RaydiumV4{}

		if err := info.Decode(res.Account.Data.GetBinary()); err != nil {
			log.Printf("decoding RaydiumV4: %w", err)
		} else {
			mres, err := clientRPC.GetAccountInfo(ctx, info.MarketID)
			if err != nil {
				log.Printf("GetPool err: %v", err)
			} else {
				if mres != nil {
					if mres.Value != nil {
						market := &MarketV3{}
						if err := market.Decode(mres.Value.Data.GetBinary()); err != nil {
							log.Printf("decoding MarketV3: %w", err)
						} else {
							pool := models.PoolConfig{
								ID:               res.Pubkey.String(),
								BaseMint:         info.BaseMint.String(),
								QuoteMint:        info.QuoteMint.String(),
								BaseDecimals:     int(info.BaseDecimal),
								QuoteDecimals:    int(info.QuoteDecimal),
								OpenOrders:       info.OpenOrders.String(),
								TargetOrders:     info.TargetOrders.String(),
								BaseVault:        info.BaseVault.String(),
								QuoteVault:       info.QuoteVault.String(),
								MarketID:         info.MarketID.String(),
								MarketBaseVault:  market.BaseVault.String(),
								MarketQuoteVault: market.QuoteVault.String(),
								MarketBids:       market.Bids.String(),
								MarketAsks:       market.Asks.String(),
								MarketEventQueue: market.EventQueue.String(),
							}
							pools = append(pools, pool)
						}
					}
				}
			}
		}
	}
	return pools, nil
}

// RaydiumV4 x
type RaydiumV4 struct {
	Status                 bin.Uint64
	Nonce                  bin.Uint64
	MaxOrder               bin.Uint64
	Depth                  bin.Uint64
	BaseDecimal            bin.Uint64
	QuoteDecimal           bin.Uint64
	State                  bin.Uint64
	ResetFlag              bin.Uint64
	MinSize                bin.Uint64
	VolMaxCutRatio         bin.Uint64
	AmountWaveRatio        bin.Uint64
	BaseLotSize            bin.Uint64
	QuoteLotSize           bin.Uint64
	MinPriceMultiplier     bin.Uint64
	MaxPriceMultiplier     bin.Uint64
	SystemDecimalValue     bin.Uint64
	MinSeparateNumerator   bin.Uint64
	MinSeparateDenominator bin.Uint64
	TradeFeeNumerator      bin.Uint64
	TradeFeeDenominator    bin.Uint64
	PnlNumerator           bin.Uint64
	PnlDenominator         bin.Uint64
	SwapFeeNumerator       bin.Uint64
	SwapFeeDenominator     bin.Uint64
	BaseNeedTakePnl        bin.Uint64
	QuoteNeedTakePnl       bin.Uint64
	QuoteTotalPnl          bin.Uint64
	BaseTotalPnl           bin.Uint64
	QuoteTotalDeposited    bin.Uint128
	BaseTotalDeposited     bin.Uint128
	SwapBaseInAmount       bin.Uint128
	SwapQuoteOutAmount     bin.Uint128
	SwapBase2QuoteFee      bin.Uint64
	SwapQuoteInAmount      bin.Uint128
	SwapBaseOutAmount      bin.Uint128
	SwapQuote2BaseFee      bin.Uint64
	BaseVault              solana.PublicKey
	QuoteVault             solana.PublicKey
	BaseMint               solana.PublicKey
	QuoteMint              solana.PublicKey
	LpMint                 solana.PublicKey
	OpenOrders             solana.PublicKey
	MarketID               solana.PublicKey
	MarketProgramID        solana.PublicKey
	TargetOrders           solana.PublicKey
	WithdrawQueue          solana.PublicKey
	LpVault                solana.PublicKey
	Owner                  solana.PublicKey
	LpReserve              bin.Uint64
	Padding                [3]byte `json:"-"`
}

// Decode x
func (m *RaydiumV4) Decode(in []byte) error {
	decoder := bin.NewBinDecoder(in)
	err := decoder.Decode(&m)
	if err != nil {
		return fmt.Errorf("unpack: %w", err)
	}
	return nil
}

// MarketV3 x
type MarketV3 struct {
	SerumPadding [5]byte `json:"-"`
	AccountFlags [8]byte `json:"-"`

	OwnAddress             solana.PublicKey
	VaultSignerNonce       bin.Uint64
	BaseMint               solana.PublicKey
	QuoteMint              solana.PublicKey
	BaseVault              solana.PublicKey
	BaseDepositsTotal      bin.Uint64
	BaseFeesAccrued        bin.Uint64
	QuoteVault             solana.PublicKey
	QuoteDepositsTotal     bin.Uint64
	QuoteFeesAccrued       bin.Uint64
	QuoteDustThreshold     bin.Uint64
	RequestQueue           solana.PublicKey
	EventQueue             solana.PublicKey
	Bids                   solana.PublicKey
	Asks                   solana.PublicKey
	BaseLotSize            bin.Uint64
	QuoteLotSize           bin.Uint64
	FeeRateBPS             bin.Uint64
	ReferrerRebatesAccrued bin.Uint64
	EndPadding             [7]byte `json:"-"`
}

// Decode x
func (m *MarketV3) Decode(in []byte) error {
	decoder := bin.NewBinDecoder(in)
	err := decoder.Decode(&m)
	if err != nil {
		return fmt.Errorf("unpack: %w", err)
	}
	return nil
}
