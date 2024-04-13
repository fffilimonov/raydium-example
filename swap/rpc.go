package swap

import (
	"context"
	"fmt"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"log"
	"time"
)

// TokenAccountInfo x
type TokenAccountInfo struct {
	Mint    solana.PublicKey
	Account solana.PublicKey
}

// GetTokenAccountsFromMints x
func GetTokenAccountsFromMints(
	clientRPC rpc.Client,
	owner solana.PublicKey,
	mints ...solana.PublicKey,
) (map[string]solana.PublicKey, map[string]solana.PublicKey, error) {

	duplicates := map[string]bool{}
	tokenAccounts := []solana.PublicKey{}
	tokenAccountInfos := []TokenAccountInfo{}

	for _, m := range mints {
		if ok := duplicates[m.String()]; ok {
			continue
		}
		duplicates[m.String()] = true
		a, _, err := solana.FindAssociatedTokenAddress(owner, m)
		if err != nil {
			return nil, nil, err
		}
		// Use owner address for NativeSOL mint
		if m.String() == "11111111111111111111111111111111" {
			a = owner
		}
		tokenAccounts = append(tokenAccounts, a)
		tokenAccountInfos = append(tokenAccountInfos, TokenAccountInfo{
			Mint:    m,
			Account: a,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	res, err := clientRPC.GetMultipleAccounts(ctx, tokenAccounts...)

	if err != nil {
		return nil, nil, err
	}

	missingAccounts := map[string]solana.PublicKey{}
	existingAccounts := map[string]solana.PublicKey{}
	for i, a := range res.Value {
		tai := tokenAccountInfos[i]
		if a == nil {
			missingAccounts[tai.Mint.String()] = tai.Account
			continue
		}
		if tai.Mint.String() == "11111111111111111111111111111111" {
			existingAccounts[tai.Mint.String()] = owner
			continue
		}
		var ta token.Account
		err = bin.NewBinDecoder(a.Data.GetBinary()).Decode(&ta)
		if err != nil {
			return nil, nil, err
		}
		existingAccounts[tai.Mint.String()] = tai.Account
	}

	return existingAccounts, missingAccounts, nil
}

// BuildTransacion x
func BuildTransacion(clientRPC *rpc.Client, signers []solana.PrivateKey, instrs ...solana.Instruction) (*solana.Transaction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	recent, err := clientRPC.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}

	tx, err := solana.NewTransaction(
		instrs,
		recent.Value.Blockhash,
		solana.TransactionPayer(signers[0].PublicKey()),
	)
	if err != nil {
		return nil, err
	}

	_, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			for _, payer := range signers {
				if payer.PublicKey().Equals(key) {
					return &payer
				}
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// ExecuteInstructionsAndWait x
func ExecuteInstructionsAndWait(
	clientRPC *rpc.Client,
	signers []solana.PrivateKey,
	instrs ...solana.Instruction,
) (*solana.Signature, error) {

	tx, err := BuildTransacion(clientRPC, signers, instrs...)
	if err != nil {
		return nil, err
	}
	log.Printf("builded")
	opts := rpc.TransactionOpts{
		SkipPreflight:       true,
		PreflightCommitment: rpc.CommitmentFinalized,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	xsig, err := clientRPC.SendTransactionWithOpts(
		ctx,
		tx,
		opts,
	)
	sig := &xsig

	if err != nil {
		log.Printf("ExecuteInstructionsAndWait: %v %v", sig, err)
		return sig, err
	}
	log.Printf("sent")

	var txErr interface{}

	for start := time.Now(); time.Since(start) < 120*time.Second; {

		ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second*20)
		defer cancel2()

		sigStatus, err := clientRPC.GetSignatureStatuses(ctx2, false, xsig)
		if err == nil {
			if sigStatus != nil && sigStatus.Value != nil && len(sigStatus.Value) > 0 && sigStatus.Value[0] != nil {
				if sigStatus.Value[0].ConfirmationStatus != rpc.ConfirmationStatusProcessed {
					txErr = sigStatus.Value[0].Err
					log.Printf("ExecuteInstructionsAndWait: %v", txErr)
					break
				}
			}
		} else {
			txErr = err
		}
		time.Sleep(5000 * time.Millisecond)
	}

	log.Printf("sent loop end: %v", txErr)

	if txErr != nil {
		return sig, fmt.Errorf("confirmation err: %v", txErr)
	}

	return sig, nil
}

// GetTokenAccountsBalance c
func GetTokenAccountsBalance(
	ctx context.Context,
	clientRPC *rpc.Client,
	accounts ...solana.PublicKey,
) (map[string]uint64, error) {
	res, err := clientRPC.GetMultipleAccounts(ctx, accounts...)
	if err != nil {
		return nil, err
	}
	tokenAccounts := map[string]uint64{}
	for i, a := range res.Value {
		if a.Owner.Equals(solana.TokenProgramID) {
			ta := token.Account{}
			err = bin.NewBinDecoder(a.Data.GetBinary()).Decode(&ta)
			if err != nil {
				return nil, err
			}
			tokenAccounts[accounts[i].String()] = ta.Amount
		} else {
			tokenAccounts[accounts[i].String()] = a.Lamports
		}
	}
	return tokenAccounts, nil
}
