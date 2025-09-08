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

	// 私钥验证
	pubKey := cfg.PrivateKey.Public()
	pubKeyECDSA, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("私钥推导公钥失败，交易失败")
	}
	from := crypto.PubkeyToAddress(*pubKeyECDSA)

	erc20ABI, _ := abi.JSON(strings.NewReader(ERC20ABI))
	routerABI, _ := abi.JSON(strings.NewReader(RouterABI))

	// 1. 获取代币精度 & 余额
	decimals, err := CallDecimals(ctx, client, erc20ABI, tokenIn)
	if err != nil {
		return fmt.Errorf("获取代币精度失败: %w", err)
	}
	amountIn := ToWeiFloat(amountHuman, int(decimals))

	tokenBal, err := CallBalanceOf(ctx, client, erc20ABI, tokenIn, from)
	if err != nil {
		return err
	}
	if amountIn.Cmp(tokenBal) > 0 {
		return fmt.Errorf("交易金额大于账户余额")
	}

	// 2. allowance 授权检查 授权Router可以提取足够代币
	allowance, _ := CallAllowance(ctx, client, erc20ABI, tokenIn, from, cfg.RouterAddr)
	nonce, _ := client.PendingNonceAt(ctx, from)
	// 建议gas费用
	gasPrice, _ := client.SuggestGasPrice(ctx)

	if allowance.Cmp(amountIn) < 0 {
		fmt.Println("approve交易认证失败，重新去获取授权")
		approveData, _ := erc20ABI.Pack("approve", cfg.RouterAddr, amountIn)

		// approve gasLimit 比较固定为80000gas 这里也可以作为配置项
		gasLimit := uint64(80000)

		// 检查 ETH 余额是否足够支付手续费
		balanceEth, err := client.BalanceAt(ctx, from, nil)
		if err != nil {
			return fmt.Errorf("获取ETH余额失败: %w", err)
		}
		requiredEth := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), gasPrice)
		if balanceEth.Cmp(requiredEth) < 0 {
			return fmt.Errorf("ETH费用少于Approve需要认证的费用")
		}

		approveTx := types.NewTransaction(nonce, tokenIn, big.NewInt(0), gasLimit, gasPrice, approveData)
		signedApprove, _ := types.SignTx(approveTx, types.NewEIP155Signer(cfg.ChainID), cfg.PrivateKey)
		if err := client.SendTransaction(ctx, signedApprove); err != nil {
			return err
		}
		fmt.Printf("Approve tx sent: %s\n", signedApprove.Hash().Hex())

		// 等待上链（异步日志监听）
		if err := WaitMinedLogs(ctx, client, signedApprove.Hash()); err != nil {
			return fmt.Errorf("交易认证失败: %w", err)
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
		return fmt.Errorf("向router获取交易费用为空")
	}
	amountOut := amounts[1]

	// 计算滑点
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
		return fmt.Errorf("获取ETH余额失败: %w", err)
	}
	requiredEth := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), gasPrice)
	if balanceEth.Cmp(requiredEth) < 0 {
		return fmt.Errorf("账户余额不足以支付gas费用")
	}

	swapTx := types.NewTransaction(nonce, cfg.RouterAddr, big.NewInt(0), gasLimit, gasPrice, swapData)
	signedSwap, _ := types.SignTx(swapTx, types.NewEIP155Signer(cfg.ChainID), cfg.PrivateKey)

	if err := client.SendTransaction(ctx, signedSwap); err != nil {
		return err
	}
	fmt.Printf("交易发送成功之后的txHash: %s\n", signedSwap.Hash().Hex())

	return WaitMinedLogs(ctx, client, signedSwap.Hash())
}
