package main

import (
	"bufio"
	"context"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/bnb48club/puissant_sdk/bnb48.sdk"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/lmittmann/w3"
	"github.com/lmittmann/w3/module/eth"
	"github.com/lmittmann/w3/w3types"
)

var (
	funcBalanceOf = w3.MustNewFunc("balanceOf(address)", "uint256")
	funcTransfer  = w3.MustNewFunc("withdrawAll(address)", "bool")
	console       = bufio.NewScanner(os.Stdin)
)

func main() {
	fbClient, err := bnb48.Dial("https://fonce-bsc.bnb48.club", "https://puissant-bsc.bnb48.club")
	if err != nil {
		log.Panicln(err.Error())
	}
	client := w3.MustDial("https://bsc-dataseed4.binance.org")
	signer := types.NewLondonSigner(big.NewInt(56))

	account, err := crypto.HexToECDSA(strings.TrimPrefix("", "0x")) // Приватник откуда делаем клейм
	if err != nil {
		log.Fatal(err)
	}
	accountAddress := crypto.PubkeyToAddress(account.PublicKey)

	donor, err := crypto.HexToECDSA(strings.TrimPrefix("", "0x")) // Приватник, откуда шлём деньги на транзу
	if err != nil {
		log.Fatal(err)
	}
	donorAddress := crypto.PubkeyToAddress(donor.PublicKey)

	contractAddress := common.HexToAddress("")   //контракт клайма
	withdrawalAddress := common.HexToAddress("") //этот адрес контракта всегда указывается при выводе

	recipient := common.HexToAddress("") // Адрес получателя

	defGasPrice := w3.I("5 gwei")

	var (
		donorNonce   uint64
		accountNonce uint64
		tokenBalance big.Int
		latestBlock  big.Int
	)

	err = client.Call(
		eth.Nonce(donorAddress, nil).Returns(&donorNonce),
		eth.Nonce(accountAddress, nil).Returns(&accountNonce),
		eth.CallFunc(funcBalanceOf, contractAddress, accountAddress).Returns(&tokenBalance),
		eth.BlockNumber().Returns(&latestBlock),
	)

	if err != nil {
		log.Fatal(err)
	}

	input, err := funcTransfer.EncodeArgs(&withdrawalAddress)
	if err != nil {
		log.Fatal(err)
	}

	gasPrice, _ := fbClient.SuggestGasPrice(context.Background())

	tokenTx := types.LegacyTx{Value: w3.Big0, To: &contractAddress, Nonce: accountNonce, Data: input, GasPrice: defGasPrice}
	var tokenTransferGas uint64
	err = client.Call(
		eth.EstimateGas(&w3types.Message{
			To:   &contractAddress,
			Func: funcTransfer,
			From: accountAddress,
			Args: []any{&withdrawalAddress},
		}, nil).Returns(&tokenTransferGas),
	)
	if err != nil {
		log.Fatal(err)
	}

	tokenTx.Gas = tokenTransferGas

	sponsorTx := types.LegacyTx{Value: new(big.Int).Mul(tokenTx.GasPrice, new(big.Int).SetUint64(tokenTx.Gas)), Gas: 21000, To: &accountAddress, Nonce: donorNonce, GasPrice: gasPrice}

	input, err = funcTransfer.EncodeArgs(&recipient, &tokenBalance)
	if err != nil {
		log.Fatal(err)
	}

	withdrawTx := types.LegacyTx{Value: w3.Big0, To: &contractAddress, Nonce: accountNonce, Data: input, GasPrice: defGasPrice, Gas: 21000}

	withdrawTxSigned, err := types.SignTx(types.NewTx(&withdrawTx), signer, account)
	if err != nil {
		log.Fatal(err)
	}

	tokenTxSigned, err := types.SignTx(types.NewTx(&tokenTx), signer, account)
	if err != nil {
		log.Fatal(err)
	}

	sponsorTxSigned, err := types.SignTx(types.NewTx(&sponsorTx), signer, donor)
	if err != nil {
		log.Fatal(err)
	}

	var rawTxs []hexutil.Bytes
	rawTxBytes, _ := rlp.EncodeToBytes(sponsorTxSigned)
	rawTxs = append(rawTxs, rawTxBytes)

	rawTxBytes, _ = rlp.EncodeToBytes(tokenTxSigned)
	rawTxs = append(rawTxs, rawTxBytes)

	rawTxBytes, _ = rlp.EncodeToBytes(withdrawTxSigned)
	rawTxs = append(rawTxs, rawTxBytes)

	log.Println(tokenTxSigned)
	_, err = fbClient.SendPuissant(context.Background(), rawTxs, uint64(time.Now().Unix()+60), nil)
	if err != nil {
		log.Println(err)
		console.Scan()
		log.Fatal(err)
	}

	log.Println("bypassed.")
	console.Scan()
}
