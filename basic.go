package main

import (
	_ "embed"
	"log"
	"main/swap"
)

func doSwap(args cliArgs) {
	fromToken := args.FromToken
	toToken := args.ToToken

	pool, reverse := swap.GetPool(clientRPC, fromToken, toToken)

	if reverse == true {
		if pool.BaseMint != toToken && pool.QuoteMint != fromToken {
			log.Fatalf("pool not found")
		}
	} else {
		if pool.BaseMint != fromToken && pool.QuoteMint != toToken {
			log.Fatalf("pool not found")
		}
	}

	swapper, err := swap.NewTokenSwapper(swap.TokenSwapperConfig{
		ClientRPC:  clientRPC,
		PrivateKey: walletPK,
		Pool:       &pool,
		Reverse:    reverse,
	})

	if err != nil {
		log.Fatalf("create swapper: %v", err)
	}

	err = swapper.Init()

	if err != nil {
		log.Fatalf("init swapper %v", err)
	}

	sig, err := swapper.Do(
		args.Amount,
		args.Slipage,
	)

	if err != nil {
		log.Fatalf("swapper do %v", err)
	}
	log.Printf("sig: %v", sig)
}
