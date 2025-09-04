package main

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"log"
)

// VerifyAndTradeService VerifyAndTrade 验证apiKey和交易 并且返回txHash
func VerifyAndTradeService(verifyAndTrade VerifyAndTrade) string {

	clint := verifyAndTrade.client
	ctx := verifyAndTrade.ctx

	privateKey, err := crypto.HexToECDSA(verifyAndTrade.privateKeyHex)
	if err != nil {
		fmt.Println("apiKey私钥验证失败！！！")
		log.Fatal(err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		fmt.Println("apiKey公钥验证失败！！！")
		log.Fatal("无效的公钥")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, _ := clint.PendingNonceAt(ctx, fromAddress)

	tx := types.NewTransaction(nonce, verifyAndTrade.targetAddress, verifyAndTrade.tradePrice, verifyAndTrade.gasLimit, verifyAndTrade.tradeGasFee, nil)
	signedTx, _ := types.SignTx(tx, types.NewEIP155Signer(verifyAndTrade.chainID), privateKey)

	err = clint.SendTransaction(ctx, signedTx)
	if err != nil {
		fmt.Println("当笔交易发送失败！！！")
		log.Fatal("交易发送失败:", err)
	}

	return signedTx.Hash().Hex()
}
