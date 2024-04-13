package swap

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"log"
	"main/models"
	"math"
	"time"
)

// ToFloat x
func ToFloat(v uint64, decimals int) float64 {
	return float64(v) / math.Pow10(decimals)
}

// FromFloat x
func FromFloat(v float64, decimals int) uint64 {
	return uint64(v * math.Pow10(decimals))
}

// RaydiumSwap x
type RaydiumSwap struct {
	clientRPC *rpc.Client
	account   solana.PrivateKey
}

// Swap x
func (s *RaydiumSwap) Swap(
	pool *models.PoolConfig,
	amount uint64,
	minOutAmount uint64,
	fromAccount solana.PublicKey,
	toAccount solana.PublicKey,
	reverse bool,
	isMissingFrom bool,
	isMissingTo bool,
) (*solana.Signature, error) {

	log.Printf("isMissingFrom: %v %v", isMissingFrom, fromAccount)
	log.Printf("isMissingTo: %v %v", isMissingTo, toAccount)

	if isMissingFrom == true && isMissingTo == true {
		return nil, fmt.Errorf("isMissingFrom and isMissingTo")
	}

	instrs := []solana.Instruction{}
	signers := []solana.PrivateKey{s.account}
	tempAccount := solana.NewWallet()

	needWrapSOL := pool.BaseMint == "So11111111111111111111111111111111111111112" || pool.QuoteMint == "So11111111111111111111111111111111111111112"
	log.Printf("need wrap0: %v", needWrapSOL)

	if needWrapSOL {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
		defer cancel()

		log.Printf("need wrap1")
		rentCost, err := s.clientRPC.GetMinimumBalanceForRentExemption(
			ctx,
			165,
			rpc.CommitmentConfirmed,
		)

		if err != nil {
			return nil, err
		}

		accountLamports := rentCost
		if reverse == false {
			if pool.BaseMint == "So11111111111111111111111111111111111111112" {
				accountLamports += amount
			}
		} else {
			if pool.QuoteMint == "So11111111111111111111111111111111111111112" {
				accountLamports += amount
			}
		}

		createInst, err := system.NewCreateAccountInstruction(
			accountLamports,
			165,
			solana.TokenProgramID,
			s.account.PublicKey(),
			tempAccount.PublicKey(),
		).ValidateAndBuild()

		if err != nil {
			return nil, err
		}

		instrs = append(instrs, createInst)
		initInst, err := token.NewInitializeAccountInstruction(
			tempAccount.PublicKey(),
			solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112"),
			s.account.PublicKey(),
			solana.SysVarRentPubkey,
		).ValidateAndBuild()

		if err != nil {
			return nil, err
		}

		instrs = append(instrs, initInst)
		signers = append(signers, tempAccount.PrivateKey)

		if reverse == false {
			// Use this new temp account as from or to
			if pool.BaseMint == "So11111111111111111111111111111111111111112" {
				fromAccount = tempAccount.PublicKey()
			}
			if pool.QuoteMint == "So11111111111111111111111111111111111111112" {
				toAccount = tempAccount.PublicKey()
			}
		} else {
			// Use this new temp account as from or to
			if pool.BaseMint == "So11111111111111111111111111111111111111112" {
				toAccount = tempAccount.PublicKey()
			}
			if pool.QuoteMint == "So11111111111111111111111111111111111111112" {
				fromAccount = tempAccount.PublicKey()
			}
		}
	}

	if isMissingFrom == true || isMissingTo == true {
		mint := pool.BaseMint
		if pool.BaseMint == "So11111111111111111111111111111111111111112" {
			mint = pool.QuoteMint
		}

		log.Printf("need to create token account: %v", mint)
		inst, err := associatedtokenaccount.NewCreateInstruction(
			s.account.PublicKey(),
			s.account.PublicKey(),
			solana.MustPublicKeyFromBase58(mint),
		).ValidateAndBuild()
		if err != nil {
			return nil, err
		}
		instrs = append(instrs, inst)
	}

	instrs = append(instrs, NewRaydiumSwapInstruction(
		amount,
		minOutAmount,
		pool,
		fromAccount,
		toAccount,
		s.account.PublicKey(),
	))

	if needWrapSOL {
		log.Printf("need wrap2")
		closeInst, err := token.NewCloseAccountInstruction(
			tempAccount.PublicKey(),
			s.account.PublicKey(),
			s.account.PublicKey(),
			[]solana.PublicKey{},
		).ValidateAndBuild()
		if err != nil {
			return nil, err
		}
		instrs = append(instrs, closeInst)
	}
	log.Printf("execute: %#v", instrs)

	sig, err := ExecuteInstructionsAndWait(s.clientRPC, signers, instrs...)
	if err != nil {
		return nil, err
	}

	return sig, nil
}

// RaySwapInstruction x
type RaySwapInstruction struct {
	bin.BaseVariant
	InAmount                uint64
	MinimumOutAmount        uint64
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

// ProgramID x
func (inst *RaySwapInstruction) ProgramID() solana.PublicKey {
	return solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")
}

// Accounts x
func (inst *RaySwapInstruction) Accounts() (out []*solana.AccountMeta) {
	return inst.Impl.(solana.AccountsGettable).GetAccounts()
}

// Data x
func (inst *RaySwapInstruction) Data() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bin.NewBorshEncoder(buf).Encode(inst); err != nil {
		return nil, fmt.Errorf("unable to encode instruction: %w", err)
	}
	return buf.Bytes(), nil
}

// MarshalWithEncoder x
func (inst *RaySwapInstruction) MarshalWithEncoder(encoder *bin.Encoder) (err error) {
	// Swap instruction is number 9
	err = encoder.WriteUint8(9)
	if err != nil {
		return err
	}
	err = encoder.WriteUint64(inst.InAmount, binary.LittleEndian)
	if err != nil {
		return err
	}
	err = encoder.WriteUint64(inst.MinimumOutAmount, binary.LittleEndian)
	if err != nil {
		return err
	}
	return nil
}

// NewRaydiumSwapInstruction x
func NewRaydiumSwapInstruction(
	inAmount uint64,
	minimumOutAmount uint64,
	pool *models.PoolConfig,
	userSourceTokenAccount solana.PublicKey,
	userDestTokenAccount solana.PublicKey,
	userOwner solana.PublicKey,
) *RaySwapInstruction {

	inst := RaySwapInstruction{
		InAmount:         inAmount,
		MinimumOutAmount: minimumOutAmount,
		AccountMetaSlice: make(solana.AccountMetaSlice, 18),
	}
	inst.BaseVariant = bin.BaseVariant{
		Impl: inst,
	}

	inst.AccountMetaSlice[0] = solana.Meta(solana.TokenProgramID)
	inst.AccountMetaSlice[1] = solana.Meta(solana.MustPublicKeyFromBase58(pool.ID)).WRITE()
	inst.AccountMetaSlice[2] = solana.Meta(solana.MustPublicKeyFromBase58("5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1"))
	inst.AccountMetaSlice[3] = solana.Meta(solana.MustPublicKeyFromBase58(pool.OpenOrders)).WRITE()
	inst.AccountMetaSlice[4] = solana.Meta(solana.MustPublicKeyFromBase58(pool.TargetOrders)).WRITE()
	inst.AccountMetaSlice[5] = solana.Meta(solana.MustPublicKeyFromBase58(pool.BaseVault)).WRITE()
	inst.AccountMetaSlice[6] = solana.Meta(solana.MustPublicKeyFromBase58(pool.QuoteVault)).WRITE()
	inst.AccountMetaSlice[7] = solana.Meta(solana.MustPublicKeyFromBase58("srmqPvymJeFKQ4zGQed1GFppgkRHL9kaELCbyksJtPX"))
	inst.AccountMetaSlice[8] = solana.Meta(solana.MustPublicKeyFromBase58(pool.MarketID)).WRITE()
	inst.AccountMetaSlice[9] = solana.Meta(solana.MustPublicKeyFromBase58(pool.MarketBids)).WRITE()
	inst.AccountMetaSlice[10] = solana.Meta(solana.MustPublicKeyFromBase58(pool.MarketAsks)).WRITE()
	inst.AccountMetaSlice[11] = solana.Meta(solana.MustPublicKeyFromBase58(pool.MarketEventQueue)).WRITE()
	inst.AccountMetaSlice[12] = solana.Meta(solana.MustPublicKeyFromBase58(pool.MarketBaseVault)).WRITE()
	inst.AccountMetaSlice[13] = solana.Meta(solana.MustPublicKeyFromBase58(pool.MarketQuoteVault)).WRITE()
	inst.AccountMetaSlice[14] = solana.Meta(solana.MustPublicKeyFromBase58("5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1"))
	inst.AccountMetaSlice[15] = solana.Meta(userSourceTokenAccount).WRITE()
	inst.AccountMetaSlice[16] = solana.Meta(userDestTokenAccount).WRITE()
	inst.AccountMetaSlice[17] = solana.Meta(userOwner).SIGNER()

	return &inst
}
