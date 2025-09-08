package main

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// RpcUrl 主网 RPC
const RpcUrl = "https://xlayerrpc.okx.com"

// RouterAddress dex router地址 UniswapV2
const RouterAddress = "0xYourRouterAddress"

// OKBAddress okb地址
const OKBAddress = "0xe538905cf8410324e03a5a23c1c177a474d59b2b"

// ERC20ABI 最小子集
const ERC20ABI = `[{"constant":true,"inputs":[{"name":"owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"},
{"constant":true,"inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"name":"allowance","outputs":[{"name":"","type":"uint256"}],"type":"function"},
{"constant":false,"inputs":[{"name":"spender","type":"address"},{"name":"value","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"type":"function"}]`

// RouterABI 最小子集（UniswapV2 风格）
const RouterABI = `[{"constant":true,"inputs":[{"name":"amountIn","type":"uint256"},{"name":"path","type":"address[]"}],"name":"getAmountsOut","outputs":[{"name":"","type":"uint256[]"}],"type":"function"},
{"constant":false,"inputs":[{"name":"amountIn","type":"uint256"},{"name":"amountOutMin","type":"uint256"},{"name":"path","type":"address[]"},{"name":"to","type":"address"},{"name":"deadline","type":"uint256"}],"name":"swapExactTokensForTokens","outputs":[{"name":"amounts","type":"uint256[]"}],"type":"function"}]`

// ToWeiFloat 把人类可读的代币数量转成最小单位
func ToWeiFloat(amount float64, decimals int) *big.Int {
	exp := new(big.Float).SetFloat64(math.Pow10(decimals))
	val := new(big.Float).Mul(new(big.Float).SetFloat64(amount), exp)
	i := new(big.Int)
	val.Int(i)
	return i
}

// CallDecimals 查询 ERC20 decimals
func CallDecimals(ctx context.Context, client *ethclient.Client, erc20ABI abi.ABI, token common.Address) (uint8, error) {
	data, _ := erc20ABI.Pack("decimals")
	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return 18, err
	}
	if len(res) == 0 {
		return 18, nil
	}
	return res[len(res)-1], nil
}

// WaitMinedLogs 通过订阅日志监听交易是否上链
func WaitMinedLogs(ctx context.Context, client *ethclient.Client, txHash common.Hash) error {
	query := ethereum.FilterQuery{}
	logsCh := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(ctx, query, logsCh)
	if err != nil {
		return fmt.Errorf("subscribe logs error: %w", err)
	}
	defer sub.Unsubscribe()

	timeout := time.After(5 * time.Minute)
	for {
		select {
		case err := <-sub.Err():
			return fmt.Errorf("log subscription error: %w", err)
		case <-timeout:
			return fmt.Errorf("tx %s not confirmed in time", txHash.Hex())
		case log := <-logsCh:
			// 每个新日志都检查是否包含目标交易
			receipt, err := client.TransactionReceipt(ctx, txHash)
			if err == nil && receipt != nil {
				if receipt.Status == types.ReceiptStatusSuccessful {
					fmt.Printf("Tx %s confirmed in block %d\n", txHash.Hex(), receipt.BlockNumber.Uint64())
					return nil
				}
				return fmt.Errorf("tx %s failed", txHash.Hex())
			}
			_ = log // 日志本身暂时不用解析
		}
	}
}

// CallBalanceOf 查询代币余额
func CallBalanceOf(ctx context.Context, client *ethclient.Client, erc20ABI abi.ABI, token, owner common.Address) (*big.Int, error) {
	data, _ := erc20ABI.Pack("balanceOf", owner)
	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	out := new(big.Int).SetBytes(res)
	return out, nil
}

// CallAllowance 查询 allowance
func CallAllowance(ctx context.Context, client *ethclient.Client, erc20ABI abi.ABI, token, owner, spender common.Address) (*big.Int, error) {
	data, _ := erc20ABI.Pack("allowance", owner, spender)
	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	out := new(big.Int).SetBytes(res)
	return out, nil
}

// ApplySlippage 计算滑点
func ApplySlippage(amount *big.Int, bp int) *big.Int {
	num := new(big.Int).Mul(amount, big.NewInt(int64(10000-bp)))
	return new(big.Int).Div(num, big.NewInt(10000))
}
