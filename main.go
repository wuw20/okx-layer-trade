package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
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

const ERC20ABI = `[{"constant":true,"inputs":[{"name":"owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"},
{"constant":true,"inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"name":"allowance","outputs":[{"name":"","type":"uint256"}],"type":"function"},
{"constant":false,"inputs":[{"name":"spender","type":"address"},{"name":"value","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"type":"function"},
{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"type":"function"}]`

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
  "name":"swapExactTokensForETH",
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

const UsdtAddr = "0x73fac6a72bdbd1c8f2b7c1c6a64d890c66f3f64e"

const FactoryABI = `[{"constant":true,"inputs":[{"name":"tokenA","type":"address"},{"name":"tokenB","type":"address"}],
"name":"getPair","outputs":[{"name":"","type":"address"}],"stateMutability":"view","type":"function"}]`

const pairABIJSON = `[
		{"constant":true,"inputs":[],"name":"token0","outputs":[{"name":"","type":"address"}],"type":"function"},
		{"constant":true,"inputs":[],"name":"token1","outputs":[{"name":"","type":"address"}],"type":"function"},
		{"constant":true,"inputs":[],"name":"getReserves","outputs":[
			{"name":"reserve0","type":"uint112"},
			{"name":"reserve1","type":"uint112"},
			{"name":"blockTimestampLast","type":"uint32"}
		],"type":"function"}
	]`

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
	TokenAddr:   "0x0cc24c51bf89c00c5affbfcf5e856c25ecbdb48e",
	OkbAddr:     "0xe538905cf8410324e03a5a23c1c177a474d59b2b",
	RouterAddr:  "0x69C236E021F5775B0D0328ded5EaC708E3B869DF",
	ChainID:     196,
	SlippageBP:  50,
	Deadline:    30,
	PrivateKey:  "0x623f230c83b4343cd0e2423a6114ca4e8a16b57f596b9ab229cbc1d9e077b7fc",
	opType:      "buy",
	amount:      "0.00001",
	AutoApprove: false,
}

type TradeInfo struct {
	PrivateKey  *ecdsa.PrivateKey
	TokenAddr   common.Address
	OkbAddr     common.Address
	RouterAddr  common.Address
	ChainID     *big.Int
	SlippageBP  int
	Deadline    time.Duration
	AutoApprove bool
}

func main() {
	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, "https://xlayerrpc.okx.com")
	if err != nil {
		log.Fatal("连接RPC失败:", err)
	}

	tokenAddrCommon := common.HexToAddress(cfg.TokenAddr)
	okbAddrCommon := common.HexToAddress(cfg.OkbAddr)

	var tokenIn, tokenOut common.Address
	if cfg.opType == "sell" {
		tokenIn = tokenAddrCommon
		tokenOut = okbAddrCommon
	} else {
		tokenIn = okbAddrCommon
		tokenOut = tokenAddrCommon
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(cfg.PrivateKey, "0x"))
	if err != nil {
		log.Fatalf("私钥解析失败: %v", err)
	}

	tradeCfg := TradeInfo{
		PrivateKey:  privateKey,
		TokenAddr:   tokenAddrCommon,
		OkbAddr:     okbAddrCommon,
		RouterAddr:  common.HexToAddress(cfg.RouterAddr),
		ChainID:     big.NewInt(cfg.ChainID),
		SlippageBP:  cfg.SlippageBP,
		Deadline:    time.Duration(cfg.Deadline) * time.Second,
		AutoApprove: cfg.AutoApprove,
	}

	amount, err := strconv.ParseFloat(cfg.amount, 64)
	if err != nil {
		log.Fatalf("金额解析失败: %v", err)
	}

	txHash, err := SwapTokens(ctx, client, tradeCfg, tokenIn, tokenOut, amount)
	if err != nil {
		log.Fatalf("交易执行失败: %v, 交易哈希: %s", err, txHash.Hex())
	}
	fmt.Println("交易执行完成")
}

func DebugPath(amountIn *big.Int, decimals int, path []common.Address) {
	fmt.Println("调试: getAmountsOut 调用参数")
	fmt.Printf("输入金额 (人类可读): %s\n", ToHuman(amountIn, decimals))
	for i, addr := range path {
		fmt.Printf("路径[%d]: %s\n", i, addr.Hex())
	}
}

func SwapTokens(ctx context.Context, client *ethclient.Client, cfg TradeInfo, tokenIn, tokenOut common.Address, amountHuman float64) (common.Hash, error) {
	pubKey := cfg.PrivateKey.Public()
	pubKeyECDSA, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return common.Hash{}, fmt.Errorf("私钥推导公钥失败")
	}
	from := crypto.PubkeyToAddress(*pubKeyECDSA)
	fmt.Println("钱包地址:", from.Hex())

	erc20ABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return common.Hash{}, fmt.Errorf("解析ERC20 ABI失败: %w", err)
	}

	routerABI, err := abi.JSON(strings.NewReader(RouterABI))
	if err != nil {
		return common.Hash{}, fmt.Errorf("解析Router ABI失败: %w", err)
	}

	// 打印Router ABI方法列表用于调试
	fmt.Println("Router ABI方法:")
	for name := range routerABI.Methods {
		fmt.Printf("- %s\n", name)
	}

	decimals, err := CallDecimals(ctx, client, erc20ABI, tokenIn)
	if err != nil {
		return common.Hash{}, fmt.Errorf("获取代币精度失败: %w", err)
	}
	amountIn := ToWeiFloat(amountHuman, int(decimals))

	balance, err := client.BalanceAt(ctx, from, nil)
	if err != nil {
		return common.Hash{}, fmt.Errorf("获取余额失败: %w", err)
	}

	if tokenIn == cfg.OkbAddr && amountIn.Cmp(balance) > 0 {
		return common.Hash{}, fmt.Errorf("OKB余额不足: 需要 %s, 实际 %s", amountIn.String(), balance.String())
	}

	_, err = client.PendingNonceAt(ctx, from)
	if err != nil {
		return common.Hash{}, fmt.Errorf("获取nonce失败: %w", err)
	}

	// 授权approve
	if tokenIn != cfg.OkbAddr {
		allowance, err := CallAllowance(ctx, client, erc20ABI, tokenIn, from, cfg.RouterAddr)
		if err != nil {
			return common.Hash{}, fmt.Errorf("查询授权失败: %w", err)
		}

		if allowance.Cmp(amountIn) < 0 {
			if cfg.AutoApprove {
				if err := AutoApproveXLayer(ctx, client, erc20ABI, tokenIn, cfg.RouterAddr, from, cfg.PrivateKey, cfg.ChainID, amountIn); err != nil {
					return common.Hash{}, fmt.Errorf("自动授权失败: %w", err)
				}
				_, err = client.PendingNonceAt(ctx, from)
				if err != nil {
					return common.Hash{}, fmt.Errorf("更新nonce失败: %w", err)
				}
			} else {
				return common.Hash{}, fmt.Errorf("授权不足: 需要 %s, 实际 %s", amountIn.String(), allowance.String())
			}
		}
	}

	factoryAddr := common.HexToAddress("0xf1cbfb1b12408dedba6dcd7bb57730baef6584fb")
	pairAddr, err := GetPairAddress(ctx, client, factoryAddr, tokenIn, tokenOut)
	fmt.Println("获取到的pair地址:", pairAddr.Hex())
	if err := InspectPair(ctx, client, pairAddr); err != nil {
		log.Fatal("检查 Pair 失败: ", err)
	}

	path := []common.Address{tokenIn, tokenOut}
	DebugPath(amountIn, int(decimals), path)

	// 检查直连池子是否存在
	err = CheckLiquidityPool(ctx, client, routerABI, cfg.RouterAddr, path)
	if err == nil {
		fmt.Printf("使用直连池子路径: %s -> %s\n", tokenIn.Hex(), tokenOut.Hex())
	}

	// 打印路径信息用于调试
	fmt.Printf("交易路径: ")
	for i, p := range path {
		if i > 0 {
			fmt.Print(" -> ")
		}
		fmt.Print(p.Hex())
	}
	fmt.Println()
	fmt.Printf("输入金额: %s (小数位: %d)\n", amountIn.String(), decimals)

	getOutData, err := routerABI.Pack("getAmountsOut", amountIn, path)
	if err != nil {
		return common.Hash{}, fmt.Errorf("打包getAmountsOut失败: %w", err)
	}

	// 打印调用数据用于调试
	fmt.Printf("调用数据: %x\n", getOutData)
	fmt.Printf("调用Router: %s\n", cfg.RouterAddr.Hex())

	code, err := client.CodeAt(ctx, cfg.RouterAddr, nil)
	if err != nil {
		fmt.Printf("获取Router合约代码失败: %s\n", err)
	}

	if len(code) == 0 {
		fmt.Printf("Router地址 %s 上没有部署合约", cfg.RouterAddr.Hex())
	}

	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &cfg.RouterAddr, Data: getOutData}, nil)
	if err != nil {
		fmt.Printf("CallContract错误详情: %v\n", err)
		return common.Hash{}, fmt.Errorf("调用getAmountsOut失败: %w", err)
	}

	if out == nil {
		fmt.Println("警告: CallContract返回nil")
		return common.Hash{}, fmt.Errorf("getAmountsOut返回空数据")
	}

	// 打印原始返回数据
	fmt.Printf("原始返回数据: %x\n", out)

	var amounts []*big.Int
	err = routerABI.UnpackIntoInterface(&amounts, "getAmountsOut", out)
	if err != nil {
		return common.Hash{}, fmt.Errorf("解析getAmountsOut结果失败: %w", err)
	}
	if len(amounts) < 2 {
		return common.Hash{}, fmt.Errorf("无效的报价结果")
	}
	amountOut := amounts[len(amounts)-1]
	amountOutMin := ApplySlippage(amountOut, cfg.SlippageBP)

	fmt.Printf("滑点: %d BP (%.2f%%)\n", cfg.SlippageBP, float64(cfg.SlippageBP)/100.0)

	deadline := big.NewInt(time.Now().Add(cfg.Deadline).Unix())
	var swapData []byte

	switch {
	case tokenIn == cfg.OkbAddr:
		swapData, err = routerABI.Pack("swapExactETHForTokens", amountOutMin, path, from, deadline)
	case tokenOut == cfg.OkbAddr:
		swapData, err = routerABI.Pack("swapExactTokensForETH", amountIn, amountOutMin, path, from, deadline)
	default:
		swapData, err = routerABI.Pack("swapExactTokensForTokens", amountIn, amountOutMin, path, from, deadline)
	}
	if err != nil {
		return common.Hash{}, fmt.Errorf("构造交易数据失败: %w", err)
	}

	msg := ethereum.CallMsg{
		From: from,
		To:   &cfg.RouterAddr,
		Data: swapData,
	}
	if tokenIn == cfg.OkbAddr {
		msg.Value = amountIn
	}

	gasLimit, err := client.EstimateGas(ctx, msg)
	if err != nil {
		gasLimit = 500000
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("获取Gas价格失败: %w", err)
	}

	requiredGas := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
	if balance.Cmp(requiredGas) < 0 {
		return common.Hash{}, fmt.Errorf("gas费不足: 需要 %s OKB", ToHuman(requiredGas, 18))
	}

	return common.Hash{}, nil
	//var txValue *big.Int
	//if tokenIn == cfg.OkbAddr {
	//	txValue = amountIn
	//} else {
	//	txValue = big.NewInt(0)
	//}
	//
	//swapTx := types.NewTransaction(nonce, cfg.RouterAddr, txValue, gasLimit, gasPrice, swapData)
	//signedSwap, err := types.SignTx(swapTx, types.NewEIP155Signer(cfg.ChainID), cfg.PrivateKey)
	//if err != nil {
	//	return common.Hash{}, fmt.Errorf("交易签名失败: %w", err)
	//}
	//
	//fmt.Printf("已签名交易: %v\n", signedSwap)
	//
	//if err := client.SendTransaction(ctx, signedSwap); err != nil {
	//	return signedSwap.Hash(), fmt.Errorf("发送交易失败: %w", err)
	//}
	//
	//fmt.Printf("交易已发送: %s\n", signedSwap.Hash().Hex())
	//if err := WaitMinedLogs(ctx, client, signedSwap.Hash()); err != nil {
	//	return signedSwap.Hash(), err
	//}
	//return signedSwap.Hash(), nil
}

// CheckLiquidityPool 检查流动性池是否存在
func CheckLiquidityPool(ctx context.Context, client *ethclient.Client, routerABI abi.ABI, routerAddr common.Address, path []common.Address) error {
	// 使用最小金额测试流动性池
	testAmount := big.NewInt(1)
	getOutData, err := routerABI.Pack("getAmountsOut", testAmount, path)
	if err != nil {
		return fmt.Errorf("打包测试数据失败: %w", err)
	}

	out, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &routerAddr,
		Data: getOutData,
	}, nil)

	if err != nil {
		return fmt.Errorf("流动性池检查失败: %w", err)
	}

	if len(out) == 0 {
		return fmt.Errorf("流动性池不存在或无效")
	}

	return nil
}

func ToWeiFloat(amount float64, decimals int) *big.Int {
	base := new(big.Float).SetFloat64(math.Pow10(decimals))
	value := new(big.Float).Mul(big.NewFloat(amount), base)
	result := new(big.Int)
	value.Int(result)
	return result
}

func ToHuman(amount *big.Int, decimals int) string {
	base := new(big.Float).SetFloat64(math.Pow10(decimals))
	value := new(big.Float).Quo(new(big.Float).SetInt(amount), base)
	return value.Text('f', decimals)
}

func CallDecimals(ctx context.Context, client *ethclient.Client, erc20ABI abi.ABI, token common.Address) (uint8, error) {
	data, err := erc20ABI.Pack("decimals")
	if err != nil {
		return 18, err
	}

	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return 18, err
	}

	var decimals uint8
	err = erc20ABI.UnpackIntoInterface(&decimals, "decimals", res)
	return decimals, err
}

func CallAllowance(ctx context.Context, client *ethclient.Client, erc20ABI abi.ABI, token, owner, spender common.Address) (*big.Int, error) {
	data, err := erc20ABI.Pack("allowance", owner, spender)
	if err != nil {
		return nil, err
	}

	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, err
	}

	allowance := new(big.Int).SetBytes(res)
	return allowance, nil
}

func ApplySlippage(amount *big.Int, bp int) *big.Int {
	adjusted := new(big.Int).Mul(amount, big.NewInt(int64(10000-bp)))
	return new(big.Int).Div(adjusted, big.NewInt(10000))
}

func AutoApproveXLayer(ctx context.Context, client *ethclient.Client, erc20ABI abi.ABI, tokenAddr, routerAddr, owner common.Address, privateKey *ecdsa.PrivateKey, chainID *big.Int, requiredAmount *big.Int) error {
	maxUint256 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1))
	approveData, err := erc20ABI.Pack("approve", routerAddr, maxUint256)
	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(ctx, owner)
	if err != nil {
		return err
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return err
	}

	msg := ethereum.CallMsg{
		From: owner,
		To:   &tokenAddr,
		Data: approveData,
	}
	gasLimit, err := client.EstimateGas(ctx, msg)
	if err != nil {
		gasLimit = 60000
	}

	tx := types.NewTransaction(nonce, tokenAddr, big.NewInt(0), gasLimit, gasPrice, approveData)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return err
	}

	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return err
	}

	fmt.Printf("授权交易已发送: %s\n", signedTx.Hash().Hex())
	return WaitMinedLogs(ctx, client, signedTx.Hash())
}

func WaitMinedLogs(ctx context.Context, client *ethclient.Client, txHash common.Hash) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)
	startTime := time.Now()

	for {
		select {
		case <-ticker.C:
			waitedSeconds := int(time.Since(startTime).Seconds())
			fmt.Printf("等待确认中... 已等候 %ds\n", waitedSeconds)

			receipt, err := client.TransactionReceipt(ctx, txHash)
			if errors.Is(err, ethereum.NotFound) {
				continue
			}
			if err != nil {
				return err
			}

			if receipt.Status == types.ReceiptStatusSuccessful {
				fmt.Printf("交易已确认: 区块 %d\n", receipt.BlockNumber.Uint64())
				return nil
			}
			return fmt.Errorf("交易失败: 状态码 %d", receipt.Status)

		case <-timeout:
			return fmt.Errorf("交易确认超时")
		}
	}
}

func GetPairAddress(ctx context.Context, client *ethclient.Client, factoryAddr, tokenA, tokenB common.Address) (common.Address, error) {
	parsedABI, err := abi.JSON(strings.NewReader(FactoryABI))
	if err != nil {
		return common.Address{}, err
	}

	data, err := parsedABI.Pack("getPair", tokenA, tokenB)
	if err != nil {
		return common.Address{}, err
	}

	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &factoryAddr, Data: data}, nil)
	if err != nil {
		return common.Address{}, err
	}

	var pair common.Address
	err = parsedABI.UnpackIntoInterface(&pair, "getPair", out)
	return pair, err
}

func InspectPair(ctx context.Context, client *ethclient.Client, pairAddr common.Address) error {

	pairABI, err := abi.JSON(strings.NewReader(pairABIJSON))
	if err != nil {
		return fmt.Errorf("解析Pair ABI失败: %w", err)
	}

	// 调用 token0
	token0Data, err := pairABI.Pack("token0")
	if err != nil {
		return fmt.Errorf("打包token0调用失败: %w", err)
	}
	token0Res, err := client.CallContract(ctx, ethereum.CallMsg{To: &pairAddr, Data: token0Data}, nil)
	if err != nil {
		return fmt.Errorf("调用token0失败: %w", err)
	}
	var token0 common.Address
	err = pairABI.UnpackIntoInterface(&token0, "token0", token0Res)
	if err != nil {
		return fmt.Errorf("解析token0返回失败: %w", err)
	}

	// 调用 token1
	token1Data, err := pairABI.Pack("token1")
	if err != nil {
		return fmt.Errorf("打包token1调用失败: %w", err)
	}
	token1Res, err := client.CallContract(ctx, ethereum.CallMsg{To: &pairAddr, Data: token1Data}, nil)
	if err != nil {
		return fmt.Errorf("调用token1失败: %w", err)
	}
	var token1 common.Address
	err = pairABI.UnpackIntoInterface(&token1, "token1", token1Res)
	if err != nil {
		return fmt.Errorf("解析token1返回失败: %w", err)
	}

	// 调用 getReserves
	getReservesData, err := pairABI.Pack("getReserves")
	if err != nil {
		return fmt.Errorf("打包getReserves调用失败: %w", err)
	}
	reservesRes, err := client.CallContract(ctx, ethereum.CallMsg{To: &pairAddr, Data: getReservesData}, nil)
	if err != nil {
		return fmt.Errorf("调用getReserves失败: %w", err)
	}
	var reserves struct {
		Reserve0           *big.Int
		Reserve1           *big.Int
		BlockTimestampLast uint32
	}
	err = pairABI.UnpackIntoInterface(&reserves, "getReserves", reservesRes)
	if err != nil {
		return fmt.Errorf("解析getReserves返回失败: %w", err)
	}

	fmt.Printf("Pair地址: %s\n", pairAddr.Hex())
	fmt.Printf("token0: %s\n", token0.Hex())
	fmt.Printf("token1: %s\n", token1.Hex())
	fmt.Printf("reserve0: %s\n", reserves.Reserve0.String())
	fmt.Printf("reserve1: %s\n", reserves.Reserve1.String())
	fmt.Printf("blockTimestampLast: %d\n", reserves.BlockTimestampLast)

	return nil
}
