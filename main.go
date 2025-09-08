package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"log"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
)

var cfg = struct {
	TokenAddr  string
	OkbAddr    string
	RouterAddr string
	ChainID    int64
	SlippageBP int
	Deadline   int
	PrivateKey string
	opType     string
	amount     string
}{
	TokenAddr:  "0x0cc24c51bf89c00c5affbfcf5e856c25ecbdb48e",                         // XDOG代币地址
	OkbAddr:    "0xe538905cf8410324e03a5a23c1c177a474d59b2b",                         // okb地址
	RouterAddr: "0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D",                         // router地址
	ChainID:    196,                                                                  // x layer
	SlippageBP: 50,                                                                   // 设置的滑点
	Deadline:   30,                                                                   // 超时时间30s
	PrivateKey: "0x623f230c83b4343cd0e2423a6114ca4e8a16b57f596b9ab229cbc1d9e077b7fc", // okx申请的私钥
	opType:     "buy",                                                                // 交易类型
	amount:     "0.00001",                                                            // 交易金额
}

// TradeInfo 交易入参
type TradeInfo struct {
	PrivateKey *ecdsa.PrivateKey // 钱包私钥
	TokenAddr  common.Address    // 代币地址
	OkbAddr    common.Address    // okb地址
	RouterAddr common.Address    // router地址
	ChainID    *big.Int          // x layer地址
	SlippageBP int               // 滑点
	Deadline   time.Duration     // 超时时间
}

func main() {

	// 连接 RPC 节点
	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, "https://xlayerrpc.okx.com")
	if err != nil {
		log.Fatal("连接 RPC 失败:", err)
	}
	fmt.Println("已连接到 X Layer 节点")

	// 打印操作信息
	// 根据操作类型选择路径
	tokenAddrCommon := common.HexToAddress(cfg.TokenAddr)
	okbAddrCommon := common.HexToAddress(cfg.OkbAddr)

	var tokenIn, tokenOut common.Address
	if cfg.opType == "sell" {
		tokenIn = tokenAddrCommon
		tokenOut = okbAddrCommon
	} else { // buy
		tokenIn = okbAddrCommon
		tokenOut = tokenAddrCommon
	}

	// 加载私钥
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(cfg.PrivateKey, "0x"))
	if err != nil {
		log.Fatalf("私钥解析失败: %v", err)
	}

	sellCfg := TradeInfo{
		PrivateKey: privateKey,
		TokenAddr:  common.HexToAddress(cfg.TokenAddr),
		OkbAddr:    common.HexToAddress(cfg.OkbAddr),
		RouterAddr: common.HexToAddress(cfg.RouterAddr),
		ChainID:    big.NewInt(cfg.ChainID),
		SlippageBP: cfg.SlippageBP,
		Deadline:   time.Duration(cfg.Deadline) * time.Second,
	}

	// 调用 SwapTokens
	amount, err := strconv.ParseFloat(cfg.amount, 64)
	err = SwapTokens(ctx, client, sellCfg, tokenIn, tokenOut, amount)
	if err != nil {
		log.Fatalf("交易执行失败: %v", err)
	}
	fmt.Println("交易执行完成。")
}

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

	return nil

	//swapTx := types.NewTransaction(nonce, cfg.RouterAddr, big.NewInt(0), gasLimit, gasPrice, swapData)
	//signedSwap, _ := types.SignTx(swapTx, types.NewEIP155Signer(cfg.ChainID), cfg.PrivateKey)
	//
	//if err := client.SendTransaction(ctx, signedSwap); err != nil {
	//	return err
	//}
	//fmt.Printf("交易发送成功之后的txHash: %s\n", signedSwap.Hash().Hex())
	//
	//return WaitMinedLogs(ctx, client, signedSwap.Hash())
}

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
