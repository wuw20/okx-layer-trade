package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// SwapTokens 将输入代币兑换成输出代币
// 参数 amountHuman 表示人类可读的输入代币数量
func SwapTokens(ctx context.Context, client *ethclient.Client, cfg TradeInfo, tokenIn, tokenOut common.Address, amountHuman float64) error {

	pubKey := cfg.PrivateKey.Public()
	pubKeyECDSA, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("invalid public key type")
	}
	from := crypto.PubkeyToAddress(*pubKeyECDSA)

	erc20ABI, _ := abi.JSON(strings.NewReader(ERC20ABI))
	routerABI, _ := abi.JSON(strings.NewReader(RouterABI))

	// 1. 获取代币精度 & 余额
	decimals, err := CallDecimals(ctx, client, erc20ABI, tokenIn)
	if err != nil {
		return fmt.Errorf("get decimals error: %w", err)
	}
	amountIn := ToWeiFloat(amountHuman, int(decimals))

	tokenBal, err := CallBalanceOf(ctx, client, erc20ABI, tokenIn, from)
	if err != nil {
		return err
	}
	if amountIn.Cmp(tokenBal) > 0 {
		return fmt.Errorf("amount exceeds token balance")
	}

	// 2. allowance 授权检查 授权Router可以提取足够代币
	allowance, _ := CallAllowance(ctx, client, erc20ABI, tokenIn, from, cfg.RouterAddr)
	nonce, _ := client.PendingNonceAt(ctx, from)
	gasPrice, _ := client.SuggestGasPrice(ctx)

	if allowance.Cmp(amountIn) < 0 {
		fmt.Println("Allowance not enough, approving...")
		approveData, _ := erc20ABI.Pack("approve", cfg.RouterAddr, amountIn)

		// 计算 gasLimit
		gasLimit := uint64(80000)

		// 检查 ETH 余额是否足够支付手续费
		balanceEth, err := client.BalanceAt(ctx, from, nil)
		if err != nil {
			return fmt.Errorf("failed to get ETH balance: %w", err)
		}
		requiredEth := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), gasPrice)
		if balanceEth.Cmp(requiredEth) < 0 {
			return fmt.Errorf("insufficient ETH balance for approve transaction fee")
		}

		approveTx := types.NewTransaction(nonce, tokenIn, big.NewInt(0), gasLimit, gasPrice, approveData)
		signedApprove, _ := types.SignTx(approveTx, types.NewEIP155Signer(cfg.ChainID), cfg.PrivateKey)
		if err := client.SendTransaction(ctx, signedApprove); err != nil {
			return err
		}
		fmt.Printf("Approve tx sent: %s\n", signedApprove.Hash().Hex())

		// 等待上链（异步日志监听）
		if err := WaitMinedLogs(ctx, client, signedApprove.Hash()); err != nil {
			return fmt.Errorf("approve failed: %w", err)
		}
		nonce++
	}

	// 3. 获取 amountOutMin
	path := []common.Address{tokenIn, tokenOut}
	getOutData, _ := routerABI.Pack("getAmountsOut", amountIn, path)
	out, _ := client.CallContract(ctx, ethereum.CallMsg{To: &cfg.RouterAddr, Data: getOutData}, nil)
	var amounts []*big.Int
	_ = routerABI.UnpackIntoInterface(&amounts, "getAmountsOut", out)
	if len(amounts) < 2 {
		return fmt.Errorf("router getAmountsOut returned empty")
	}
	amountOut := amounts[1]
	amountOutMin := ApplySlippage(amountOut, cfg.SlippageBP)

	// 4. 构造 swap 交易
	deadline := big.NewInt(time.Now().Add(cfg.Deadline).Unix())
	swapData, _ := routerABI.Pack("swapExactTokensForTokens", amountIn, amountOutMin, path, from, deadline)

	msg := ethereum.CallMsg{From: from, To: &cfg.RouterAddr, Data: swapData}
	gasLimit, err := client.EstimateGas(ctx, msg)
	if err != nil {
		gasLimit = 300000
	}

	// 检查 ETH 余额是否足够支付手续费
	balanceEth, err := client.BalanceAt(ctx, from, nil)
	if err != nil {
		return fmt.Errorf("failed to get ETH balance: %w", err)
	}
	requiredEth := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), gasPrice)
	if balanceEth.Cmp(requiredEth) < 0 {
		return fmt.Errorf("insufficient ETH balance for swap transaction fee")
	}

	swapTx := types.NewTransaction(nonce, cfg.RouterAddr, big.NewInt(0), gasLimit, gasPrice, swapData)
	signedSwap, _ := types.SignTx(swapTx, types.NewEIP155Signer(cfg.ChainID), cfg.PrivateKey)

	if err := client.SendTransaction(ctx, signedSwap); err != nil {
		return err
	}
	fmt.Printf("Swap tx sent: %s\n", signedSwap.Hash().Hex())

	return WaitMinedLogs(ctx, client, signedSwap.Hash())
}
