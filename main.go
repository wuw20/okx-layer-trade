package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ERC20ABI 最小子集
const ERC20ABI = `[{"constant":true,"inputs":[{"name":"owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"},
{"constant":true,"inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"name":"allowance","outputs":[{"name":"","type":"uint256"}],"type":"function"},
{"constant":false,"inputs":[{"name":"spender","type":"address"},{"name":"value","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"type":"function"}]`

// RouterABI 最小子集
const RouterABI = `[{
  "constant": true,
  "inputs": [
    {"name": "amountIn", "type": "uint256"},
    {"name": "path", "type": "address[]"}
  ],
  "name": "getAmountsOut",
  "outputs": [
    {"name": "amounts", "type": "uint256[]"}
  ],
  "stateMutability": "view",
  "type": "function"
},
{
  "constant": false,
  "inputs": [
    {"name":"amountOutMin","type":"uint256"},
    {"name":"path","type":"address[]"},
    {"name":"to","type":"address"},
    {"name":"deadline","type":"uint256"}
  ],
  "name":"swapExactETHForTokens",
  "outputs":[{"name":"amounts","type":"uint256[]"}],
  "type":"function"
},
{
  "constant": false,
  "inputs": [
    {"name":"amountIn","type":"uint256"},
    {"name":"amountOutMin","type":"uint256"},
    {"name":"path","type":"address[]"},
    {"name":"to","type":"address"},
    {"name":"deadline","type":"uint256"}
  ],
  "name":"swapExactTokensForTokens",
  "outputs":[{"name":"amounts","type":"uint256[]"}],
  "type":"function"
}]`

var cfg = struct {
	TokenAddr   string
	OkbAddr     string
	RouterAddr  string
	ChainID     int64
	SlippageBP  int
	Deadline    int
	PrivateKey  string
	opType      string
	amount      string
	AutoApprove bool
}{
	TokenAddr:   "0x0cc24c51bf89c00c5affbfcf5e856c25ecbdb48e",                         // XDOG代币地址
	OkbAddr:     "0xe538905cf8410324e03a5a23c1c177a474d59b2b",                         // wOkb地址
	RouterAddr:  "0xcc6C3FB4a9Dc710c4a187D2c5CD032c7F5086f0a",                         // xlayer router地址
	ChainID:     196,                                                                  // x layer
	SlippageBP:  50,                                                                   // 设置的滑点
	Deadline:    30,                                                                   // 超时时间30s
	PrivateKey:  "0x623f230c83b4343cd0e2423a6114ca4e8a16b57f596b9ab229cbc1d9e077b7fc", // okx申请的私钥
	opType:      "buy",                                                                // 交易类型
	amount:      "0.0000001",                                                          // 交易金额
	AutoApprove: false,
}

// TradeInfo 交易入参
type TradeInfo struct {
	PrivateKey  *ecdsa.PrivateKey // 钱包私钥
	TokenAddr   common.Address    // 代币地址
	OkbAddr     common.Address    // okb地址
	RouterAddr  common.Address    // router地址
	ChainID     *big.Int          // x layer地址
	SlippageBP  int               // 滑点
	Deadline    time.Duration     // 超时时间
	AutoApprove bool              // 是否开启授权
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
		PrivateKey:  privateKey,
		TokenAddr:   common.HexToAddress(cfg.TokenAddr),
		OkbAddr:     common.HexToAddress(cfg.OkbAddr),
		RouterAddr:  common.HexToAddress(cfg.RouterAddr),
		ChainID:     big.NewInt(cfg.ChainID),
		SlippageBP:  cfg.SlippageBP,
		Deadline:    time.Duration(cfg.Deadline) * time.Second,
		AutoApprove: cfg.AutoApprove,
	}

	// 输入金额转化
	amount, err := strconv.ParseFloat(cfg.amount, 64)
	// 调用 SwapTokens
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
	fmt.Println("Derived wallet address:", from.Hex())

	erc20ABI, _ := abi.JSON(strings.NewReader(ERC20ABI))
	routerABI, _ := abi.JSON(strings.NewReader(RouterABI))

	// 1. 获取代币精度&余额 okb精度&余额
	decimals, err := CallDecimals(ctx, client, erc20ABI, tokenIn)
	if err != nil {
		return fmt.Errorf("获取代币精度失败: %w", err)
	}
	amountIn := ToWeiFloat(amountHuman, int(decimals))

	// 获取okb余额
	balanceOKB, err := client.BalanceAt(ctx, from, nil)
	if err != nil {
		return err
	}
	if amountIn.Cmp(balanceOKB) > 0 {
		return fmt.Errorf("交易金额大于账户余额: %w, %w", balanceOKB, amountIn)
	}

	// 2. allowance 授权检查 授权Router可以提取足够代币
	allowance, _ := CallAllowance(ctx, client, erc20ABI, tokenIn, from, cfg.RouterAddr)
	nonce, _ := client.PendingNonceAt(ctx, from)

	if allowance.Cmp(amountIn) < 0 {
		if cfg.AutoApprove {
			if err := AutoApproveXLayer(ctx, client, erc20ABI, tokenIn, cfg.RouterAddr, from, cfg.PrivateKey, cfg.ChainID, amountIn); err != nil {
				return err
			}
			nonce, _ = client.PendingNonceAt(ctx, from) // 更新 nonce
		} else {
			return fmt.Errorf("需要交易的金额大于授权Router可提取的金额: allowance=%s, amountIn=%s", allowance.String(), amountIn.String())
		}
	}

	// 3. 获取 amountOutMin 交易滑点
	path := []common.Address{tokenIn, tokenOut}
	fmt.Printf("Calling getAmountsOut, amountIn=%s, path=%v\n", amountIn.String(), path)

	getOutData, _ := routerABI.Pack("getAmountsOut", amountIn, path)
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &cfg.RouterAddr, Data: getOutData}, nil)
	if err != nil {
		return fmt.Errorf("call getAmountsOut failed: %w", err)
	}
	if len(out) == 0 {
		return fmt.Errorf("getAmountsOut 返回为空，请检查 Router 地址或交易对是否存在流动性")
	}

	var amounts []*big.Int
	err = routerABI.UnpackIntoInterface(&amounts, "getAmountsOut", out)
	if err != nil {
		return fmt.Errorf("解包失败: %v", err)
	}
	if len(amounts) < 2 {
		return fmt.Errorf("向router获取交易费用为空")
	}
	amountOut := amounts[1]

	// 计算滑点
	amountOutMin := ApplySlippage(amountOut, cfg.SlippageBP)

	// 4. 构造 swap 交易
	deadline := big.NewInt(time.Now().Add(cfg.Deadline).Unix())

	var swapData []byte
	if tokenIn == cfg.OkbAddr {
		// 用原生OKB买代币
		swapData, _ = routerABI.Pack("swapExactETHForTokens", amountOutMin, path, from, deadline)
	} else if tokenOut == cfg.OkbAddr {
		// 卖代币换原生OKB
		swapData, _ = routerABI.Pack("swapExactTokensForETH", amountIn, amountOutMin, path, from, deadline)
	} else {
		// 代币之间互换
		swapData, _ = routerABI.Pack("swapExactTokensForTokens", amountIn, amountOutMin, path, from, deadline)
	}

	msg := ethereum.CallMsg{From: from, To: &cfg.RouterAddr, Data: swapData}
	gasLimit, err := client.EstimateGas(ctx, msg)
	if err != nil {
		gasLimit = 300000
	}

	// 检查OKB余额是否足够支付手续费
	// 建议gas费用
	gasPrice, _ := client.SuggestGasPrice(ctx)
	requiredOKB := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), gasPrice)
	if balanceOKB.Cmp(requiredOKB) < 0 {
		return fmt.Errorf("账户余额不足以支付gas费用")
	}

	var txValue *big.Int
	if tokenIn == cfg.OkbAddr {
		txValue = amountIn
	} else {
		txValue = big.NewInt(0)
	}
	swapTx := types.NewTransaction(nonce, cfg.RouterAddr, txValue, gasLimit, gasPrice, swapData)
	signedSwap, _ := types.SignTx(swapTx, types.NewEIP155Signer(cfg.ChainID), cfg.PrivateKey)

	fmt.Println("signedSwap: %w", signedSwap)
	return nil
	//if err := client.SendTransaction(ctx, signedSwap); err != nil {
	//	return err
	//}
	//
	//fmt.Printf("交易发送成功之后的txHash: %s\n", signedSwap.Hash().Hex())
	//return WaitMinedLogs(ctx, client, signedSwap.Hash())
}

// TokenBalance 代币余额结构
type TokenBalance struct {
	Amount   *big.Int // 余额（原始整数，未除以精度）
	Decimals int      // 代币精度
}

// GetWalletBalances 查询钱包的原生币(OKB)和指定代币余额
func GetWalletBalances(ctx context.Context, client *ethclient.Client, owner common.Address, tokens map[string]common.Address) (map[string]TokenBalance, error) {
	balances := make(map[string]TokenBalance)

	// 1. 查询原生 OKB (链上的 ETH)
	balEth, err := client.BalanceAt(ctx, owner, nil)
	if err != nil {
		return nil, fmt.Errorf("查询原生OKB失败: %w", err)
	}
	balances["OKB"] = TokenBalance{
		Amount:   balEth,
		Decimals: 18, // OKB 作为原生币，固定18位
	}

	// 2. 查询 ERC20 代币
	erc20ABI, _ := abi.JSON(strings.NewReader(ERC20ABI))
	for name, tokenAddr := range tokens {
		// balanceOf
		data, _ := erc20ABI.Pack("balanceOf", owner)
		res, err := client.CallContract(ctx, ethereum.CallMsg{To: &tokenAddr, Data: data}, nil)
		if err != nil {
			balances[name] = TokenBalance{Amount: big.NewInt(0), Decimals: 18}
			continue
		}
		bal := new(big.Int).SetBytes(res)

		// decimals
		dataDec, _ := erc20ABI.Pack("decimals")
		resDec, err := client.CallContract(ctx, ethereum.CallMsg{To: &tokenAddr, Data: dataDec}, nil)
		decimals := 18
		if err == nil && len(resDec) > 0 {
			decimals = int(resDec[len(resDec)-1])
		}

		balances[name] = TokenBalance{Amount: bal, Decimals: decimals}
	}

	return balances, nil
}

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

// AutoApproveXLayer 检查 allowance，如果不足则自动发送 approve 交易
func AutoApproveXLayer(ctx context.Context, client *ethclient.Client, erc20ABI abi.ABI, tokenAddr, routerAddr, owner common.Address, privateKey *ecdsa.PrivateKey, chainID *big.Int, requiredAmount *big.Int) error {
	// 查询当前 allowance
	data, _ := erc20ABI.Pack("allowance", owner, routerAddr)
	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &tokenAddr, Data: data}, nil)
	if err != nil {
		return fmt.Errorf("查询 allowance 失败: %w", err)
	}
	allowance := new(big.Int).SetBytes(res)

	// 如果 allowance 足够
	if allowance.Cmp(requiredAmount) >= 0 {
		fmt.Println("Allowance 足够，无需授权")
		return nil
	}

	// 构造 approve 数据
	maxUint256 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1))
	approveData, _ := erc20ABI.Pack("approve", routerAddr, maxUint256)

	nonce, _ := client.PendingNonceAt(ctx, owner)
	gasPrice, _ := client.SuggestGasPrice(ctx)
	msg := ethereum.CallMsg{From: owner, To: &tokenAddr, Data: approveData}
	gasLimit, _ := client.EstimateGas(ctx, msg)
	if gasLimit == 0 {
		gasLimit = 60000
	}

	tx := types.NewTransaction(nonce, tokenAddr, big.NewInt(0), gasLimit, gasPrice, approveData)
	signedTx, _ := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return fmt.Errorf("发送 approve 交易失败: %w", err)
	}
	fmt.Printf("已发送 approve 交易: %s\n", signedTx.Hash().Hex())

	return nil
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
