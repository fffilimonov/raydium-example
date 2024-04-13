package swap

import (
	"context"
	"errors"
	"log"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"main/models"
)

var (
	// ErrUpdateBalances x
	ErrUpdateBalances = errors.New("failed to update wallet balances")
	// ErrFromBalanceNotEnough x
	ErrFromBalanceNotEnough = errors.New("from balance not enough for swap")
)

// TaskConfig x
type TaskConfig struct {
	amount  uint64
	slipage float64
}

// TokenSwapperConfig x
type TokenSwapperConfig struct {
	ClientRPC  *rpc.Client
	PrivateKey string
	Pool       *models.PoolConfig
	Reverse    bool
}

// TokenSwapper x
type TokenSwapper struct {
	clientRPC       *rpc.Client
	account         solana.PrivateKey
	raydiumSwap     *RaydiumSwap
	tokenAccounts   map[string]solana.PublicKey
	swapTask        TaskConfig
	pool            *models.PoolConfig
	reverse         bool
	missingAccounts map[string]solana.PublicKey
	IsMissingFrom   bool
	IsMissingTo     bool
}

// GetPublic x
func (s *TokenSwapper) GetPublic() string {
	return s.account.PublicKey().String()
}

// Init x
func (s *TokenSwapper) Init() error {

	mints := []solana.PublicKey{}

	if s.pool.BaseMint == "So11111111111111111111111111111111111111112" {
		mints = append(mints, solana.MustPublicKeyFromBase58("11111111111111111111111111111111"))
	} else {
		mints = append(mints, solana.MustPublicKeyFromBase58(s.pool.BaseMint))
	}

	if s.pool.QuoteMint == "So11111111111111111111111111111111111111112" {
		mints = append(mints, solana.MustPublicKeyFromBase58("11111111111111111111111111111111"))
	} else {
		mints = append(mints, solana.MustPublicKeyFromBase58(s.pool.QuoteMint))
	}

	existingAccounts, missingAccounts, err := GetTokenAccountsFromMints(*s.clientRPC, s.account.PublicKey(), mints...)
	if err != nil {
		return err
	}
	s.missingAccounts = missingAccounts

	s.tokenAccounts = existingAccounts

	return nil
}

// Do x
func (s *TokenSwapper) Do(
	xamount float64,
	slipage float64,
) (*solana.Signature, error) {

	_, _, mam, fromAddress, toAddress, err := s.Estimate(xamount, slipage)

	sig, err := s.raydiumSwap.Swap(
		s.pool,
		s.swapTask.amount,
		mam,
		fromAddress,
		toAddress,
		s.reverse,
		s.IsMissingFrom,
		s.IsMissingTo,
	)

	if err != nil {
		return sig, err
	}

	return sig, nil
}

// Estimate x
func (s *TokenSwapper) Estimate(
	xamount float64,
	slipage float64,
) (float64, float64, uint64, solana.PublicKey, solana.PublicKey, error) {
	var minimumOutAmount float64 = 0.0
	var amount uint64 = 0
	var mam uint64 = 0
	var estimated = 0.0

	if s.reverse == false {
		amount = FromFloat(xamount, s.pool.BaseDecimals)
	} else {
		amount = FromFloat(xamount, s.pool.QuoteDecimals)
	}

	s.swapTask = TaskConfig{
		amount:  amount,
		slipage: slipage,
	}

	bm := s.pool.BaseMint
	if bm == "So11111111111111111111111111111111111111112" {
		bm = "11111111111111111111111111111111"
	}

	qm := s.pool.QuoteMint
	if qm == "So11111111111111111111111111111111111111112" {
		qm = "11111111111111111111111111111111"
	}

	fromToken := bm
	toToken := qm

	if s.reverse == true {
		fromToken = qm
		toToken = bm
	}

	var fromAddress solana.PublicKey
	missingFrom, ok := s.missingAccounts[fromToken]
	if ok {
		log.Printf("missingFrom ok: %#v", fromToken, missingFrom)
		fromAddress = missingFrom
		s.IsMissingFrom = true
	} else {
		fromAddress = s.tokenAccounts[fromToken]
	}

	var toAddress solana.PublicKey
	missingTo, ok := s.missingAccounts[toToken]
	if ok {
		log.Printf("missingTo ok: %#v", toToken, missingTo)
		toAddress = missingTo
		s.IsMissingTo = true
	} else {
		toAddress = s.tokenAccounts[toToken]
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	res, err := s.clientRPC.GetMultipleAccounts(
		ctx,
		solana.MustPublicKeyFromBase58(s.pool.BaseVault),
		solana.MustPublicKeyFromBase58(s.pool.QuoteVault),
	)

	if err != nil {
		return estimated, minimumOutAmount, mam, fromAddress, toAddress, err
	}

	var baseVault token.Account
	err = bin.NewBinDecoder(res.Value[0].Data.GetBinary()).Decode(&baseVault)
	if err != nil {
		return estimated, minimumOutAmount, mam, fromAddress, toAddress, err
	}

	var quoteVault token.Account
	err = bin.NewBinDecoder(res.Value[1].Data.GetBinary()).Decode(&quoteVault)
	if err != nil {
		return estimated, minimumOutAmount, mam, fromAddress, toAddress, err
	}

	log.Printf("reverse %v", s.reverse)

	bAm := ToFloat(baseVault.Amount, s.pool.BaseDecimals)
	qAm := ToFloat(quoteVault.Amount, s.pool.QuoteDecimals)

	log.Printf("baseVault.Amount %v", baseVault.Amount)
	log.Printf("quoteVault.Amount %v", quoteVault.Amount)

	log.Printf("bAm %v", bAm)
	log.Printf("qAm %v", qAm)

	if s.reverse == false {
		am := ToFloat(s.swapTask.amount, s.pool.BaseDecimals)
		log.Printf("am %v", am)
		denominator := bAm + am
		minimumOutAmount = qAm * am / denominator
		estimated = minimumOutAmount
		minimumOutAmount = minimumOutAmount * slipage / 100.0
		mam = FromFloat(minimumOutAmount, s.pool.QuoteDecimals)
	} else {
		am := ToFloat(s.swapTask.amount, s.pool.QuoteDecimals)
		log.Printf("am %v", am)
		denominator := qAm + am
		minimumOutAmount = bAm * am / denominator
		estimated = minimumOutAmount
		minimumOutAmount = minimumOutAmount * slipage / 100.0
		mam = FromFloat(minimumOutAmount, s.pool.BaseDecimals)
	}

	log.Printf("amount %v", s.swapTask.amount)
	log.Printf("minimumOutAmount %v", minimumOutAmount)
	log.Printf("mam %v", mam)

	if mam <= 0 {
		return estimated, minimumOutAmount, mam, fromAddress, toAddress, errors.New("min swap output amount must be grater then zero, try to swap a bigger amount")
	}

	return estimated, minimumOutAmount, mam, fromAddress, toAddress, nil
}

// NewTokenSwapper x
func NewTokenSwapper(cfg TokenSwapperConfig) (*TokenSwapper, error) {

	privateKey, err := solana.PrivateKeyFromBase58(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}

	raydiumSwap := RaydiumSwap{
		clientRPC: cfg.ClientRPC,
		account:   privateKey,
	}

	l := TokenSwapper{
		clientRPC:     cfg.ClientRPC,
		account:       privateKey,
		raydiumSwap:   &raydiumSwap,
		pool:          cfg.Pool,
		reverse:       cfg.Reverse,
		IsMissingFrom: false,
		IsMissingTo:   false,
	}

	return &l, nil
}

func recoverFromPanic() {
	if r := recover(); r != nil {
		log.Printf("Recovered from panic: %v", r)
	}
}
