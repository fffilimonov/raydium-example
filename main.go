package main

import (
	_ "embed"
	"github.com/alexflint/go-arg"
	"github.com/gagliardetto/solana-go/rpc"
	"main/models"
)

var rpcURL = ""

var walletPK = ""

type cliArgs struct {
	FromToken string  `arg:"--from" help:"from"`
	ToToken   string  `arg:"--to" help:"to"`
	Amount    float64 `arg:"--amount" help:"amount"`
	Slipage   float64 `arg:"--slipage" help:"slipage"`
}

var clientRPC *rpc.Client

func main() {
	var args cliArgs
	arg.MustParse(&args)
	models.Init()
	clientRPC = rpc.New(rpcURL)

	doSwap(args)
}
