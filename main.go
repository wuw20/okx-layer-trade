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
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// XLayer网络配置（清理冗余字段）
var xlayerConfig = struct {
	RPCEndpoint    string
	RouterAddress  string // 统一路由地址 UniswapV2Router
	TokenAddr      string // 目标代币地址
	WNativeAddress string // 包装原生币（如WETH/WOKB/WXLAY）地址
	ChainID        int64
	SlippageBP     int // 滑点（基点，1/10000）
	DeadlineSec    int // 交易过期时间（秒）
	PrivateKey     string
	OpType         string // "buy" 或 "sell"
	Amount         string // 交易金额（如"0.001"）
}{
	RPCEndpoint:    "https://xlayerrpc.okx.com",
	RouterAddress:  "0x127a986cE31AA2ea8E1a6a0F0D5b7E5dbaD7b0bE",
	TokenAddr:      "0x0cc24c51bf89c00c5affbfcf5e856c25ecbdb48e",
	WNativeAddress: "0xe538905cf8410324e03a5a23c1c177a474d59b2b", // 请替换为XLayer上的WNative地址
	ChainID:        196,
	SlippageBP:     50,                                                                   // 0.5%（50/10000）
	DeadlineSec:    600,                                                                  // 10分钟过期
	PrivateKey:     "0x623f230c83b4343cd0e2423a6114ca4e8a16b57f596b9ab229cbc1d9e077b7fc", // 实际使用时需替换为安全私钥
	OpType:         "buy",
	Amount:         "0.001",
}

// 预设候选中间代币地址列表（可根据实际情况调整）
var candidateMiddleTokens = []common.Address{
	// 运行时将会把第一个元素设置为WNative（如WETH/WOKB）
	common.Address{},
	// 可添加更多候选中间代币地址（稳定币/主流代币），需替换为XLayer主网地址
}

var routerABI abi.ABI
var erc20ABI abi.ABI

func init() {
	var err error
	routerABI, err = abi.JSON(strings.NewReader(`[
        {"inputs":[{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"}],"name":"getAmountsOut","outputs":[{"internalType":"uint256[]","name":"","type":"uint256[]"}],"stateMutability":"view","type":"function"},
        {"inputs":[{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactETHForTokens","outputs":[{"internalType":"uint256[]","name":"","type":"uint256[]"}],"stateMutability":"payable","type":"function"},
        {"inputs":[{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactTokensForETH","outputs":[{"internalType":"uint256[]","name":"","type":"uint256[]"}],"stateMutability":"nonpayable","type":"function"},
        {"inputs":[{"internalType":"uint256","name":"amountIn","type":"uint256"},{"internalType":"uint256","name":"amountOutMin","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapExactTokensForTokens","outputs":[{"internalType":"uint256[]","name":"","type":"uint256[]"}],"stateMutability":"nonpayable","type":"function"}
    ]`))
	if err != nil {
		log.Fatalf("解析Router ABI失败: %v", err)
	}

	erc20ABI, err = abi.JSON(strings.NewReader(`[{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"type":"function"},{"constant":true,"inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"name":"allowance","outputs":[{"name":"","type":"uint256"}],"type":"function"},{"constant":false,"inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"type":"function"}]`))
	if err != nil {
		log.Fatalf("解析ERC20 ABI失败: %v", err)
	}
}

func main() {
	ctx := context.Background()

	client, err := ethclient.DialContext(ctx, xlayerConfig.RPCEndpoint)
	if err != nil {
		log.Fatalf("无法连接到XLayer节点: %v", err)
	}
	defer client.Close()

	privateKey, senderAddress := loadPrivateKey()
	fmt.Printf("使用地址: %s\n", senderAddress.Hex())

	amount, err := strconv.ParseFloat(xlayerConfig.Amount, 64)
	if err != nil {
		log.Fatalf("金额解析失败: %v", err)
	}

	routerAddress := common.HexToAddress(xlayerConfig.RouterAddress)
	tokenAddress := common.HexToAddress(xlayerConfig.TokenAddr)
	wnativeAddress := common.HexToAddress(xlayerConfig.WNativeAddress)

	router := bind.NewBoundContract(routerAddress, routerABI, client, client, client)
	token := bind.NewBoundContract(tokenAddress, erc20ABI, client, client, client)

	// 将候选中间代币的首位设为WNative，便于三跳路径尝试
	if len(candidateMiddleTokens) > 0 {
		candidateMiddleTokens[0] = wnativeAddress
	}

	tokenDecimals := getTokenDecimals(token)
	okbDecimals := 18 // 假设WNative精度为18（大多数是18）

	amountWei := convertAmountToWei(amount, tokenDecimals, okbDecimals, xlayerConfig.OpType)

	auth := createTransactor(client, privateKey, ctx)

	var path []common.Address
	if xlayerConfig.OpType == "buy" {
		path, err = findBestPath(router, wnativeAddress, tokenAddress, amountWei)
		if err != nil {
			log.Fatalf("寻找最佳兑换路径失败: %v", err)
		}
		fmt.Printf("最佳兑换路径 (买): %v\n", path)
	} else if xlayerConfig.OpType == "sell" {
		path, err = findBestPath(router, tokenAddress, wnativeAddress, amountWei)
		if err != nil {
			log.Fatalf("寻找最佳兑换路径失败: %v", err)
		}
		fmt.Printf("最佳兑换路径 (卖): %v\n", path)
	} else {
		log.Fatalf("不支持的交易类型: %s", xlayerConfig.OpType)
	}

	executeSwap(client, auth, router, token, senderAddress, amountWei, path, xlayerConfig.SlippageBP, xlayerConfig.DeadlineSec, xlayerConfig.OpType)
}

// loadPrivateKey 解析私钥并返回私钥对象及对应地址
func loadPrivateKey() (*ecdsa.PrivateKey, common.Address) {
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(xlayerConfig.PrivateKey, "0x"))
	if err != nil {
		log.Fatalf("私钥解析错误: %v", err)
	}
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	senderAddress := crypto.PubkeyToAddress(*publicKey)
	return privateKey, senderAddress
}

// getTokenDecimals 获取代币精度，失败则默认18
func getTokenDecimals(token *bind.BoundContract) int {
	// ERC20 decimals 返回 uint8, 但 bind.Call 支持返回 []interface{}
	var out []interface{}
	err := token.Call(&bind.CallOpts{}, &out, "decimals")
	if err != nil || len(out) == 0 {
		log.Printf("获取代币精度失败，默认使用18: %v", err)
		return 18
	}
	// 类型断言为 uint8
	decimals, ok := out[0].(uint8)
	if !ok {
		log.Printf("解析代币精度失败，默认使用18")
		return 18
	}
	return int(decimals)
}

// convertAmountToWei 根据操作类型转换金额为Wei单位
func convertAmountToWei(amount float64, tokenDecimals, okbDecimals int, opType string) *big.Int {
	if opType == "buy" {
		return ToWeiFloat(amount, okbDecimals)
	}
	return ToWeiFloat(amount, tokenDecimals)
}

// createTransactor 创建交易授权者，自动估算GasPrice并设置链ID
func createTransactor(client *ethclient.Client, privateKey *ecdsa.PrivateKey, ctx context.Context) *bind.TransactOpts {
	auth, err := newTransactor(client, privateKey)
	if err != nil {
		log.Fatalf("创建交易授权者失败: %v", err)
	}
	auth.Context = ctx
	auth.GasLimit = 0 // 0表示自动估算
	return auth
}

// executeSwap 根据交易类型执行买入或卖出操作
func executeSwap(client *ethclient.Client, auth *bind.TransactOpts, router *bind.BoundContract, token *bind.BoundContract, sender common.Address, amountIn *big.Int, path []common.Address, slippageBP, deadlineSec int, opType string) {
	if len(path) < 2 {
		log.Fatalf("无效的交易路径")
		return
	}
	wnative := common.HexToAddress(xlayerConfig.WNativeAddress)
	from := path[0]
	to := path[len(path)-1]

	switch {
	case from == wnative && opType == "buy":
		log.Println("开始执行买入操作...")
		if err := swapExactETHForTokens(client, auth, router, sender, amountIn, path, slippageBP, deadlineSec); err != nil {
			log.Fatalf("购买代币失败: %v", err)
		}
	case to == wnative && opType == "sell":
		log.Println("开始执行卖出操作，先授权代币...")
		if err := approveTokenAndWait(client, auth, token, common.HexToAddress(xlayerConfig.RouterAddress), amountIn); err != nil {
			log.Fatalf("授权代币失败: %v", err)
		}
		if err := swapExactTokensForETH(client, auth, router, sender, amountIn, path, slippageBP, deadlineSec); err != nil {
			log.Fatalf("出售代币失败: %v", err)
		}
	default:
		// Token -> Token 路径，使用 T4T
		log.Println("执行代币对代币兑换，先授权输入代币...")
		// 这里需要使用 path[0] 对应的代币合约做授权
		inputToken := bind.NewBoundContract(from, erc20ABI, client, client, client)
		if err := approveTokenAndWait(client, auth, inputToken, common.HexToAddress(xlayerConfig.RouterAddress), amountIn); err != nil {
			log.Fatalf("授权输入代币失败: %v", err)
		}
		if err := swapExactTokensForTokens(client, auth, router, sender, amountIn, path, slippageBP, deadlineSec); err != nil {
			log.Fatalf("代币对代币兑换失败: %v", err)
		}
	}
}

// ToWeiFloat 将float金额转换为Wei单位（根据精度）
func ToWeiFloat(amount float64, decimals int) *big.Int {
	base := new(big.Float).SetFloat64(math.Pow10(decimals))
	value := new(big.Float).Mul(big.NewFloat(amount), base)
	result := new(big.Int)
	value.Int(result)
	return result
}

// newTransactor 创建带链ID和GasPrice的交易授权者
func newTransactor(client *ethclient.Client, privateKey *ecdsa.PrivateKey) (*bind.TransactOpts, error) {
	chainID := big.NewInt(xlayerConfig.ChainID)
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return nil, err
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, err
	}
	// 提高Gas价格加速交易（如1.2倍）
	gasPrice = new(big.Int).Mul(gasPrice, big.NewInt(120))
	gasPrice.Div(gasPrice, big.NewInt(100))
	auth.GasPrice = gasPrice

	return auth, nil
}

// calculateSlippageAmount 计算滑点后的最小接收量
func calculateSlippageAmount(estimatedOut *big.Int, slippageBP int) *big.Int {
	slippage := new(big.Int).Mul(estimatedOut, big.NewInt(int64(slippageBP)))
	slippage.Div(slippage, big.NewInt(10000))
	amountOutMin := new(big.Int).Sub(estimatedOut, slippage)
	if amountOutMin.Cmp(big.NewInt(0)) < 0 {
		amountOutMin = big.NewInt(0)
	}
	return amountOutMin
}

// waitTransactionConfirmation 等待交易确认并检查状态
func waitTransactionConfirmation(ctx context.Context, client *ethclient.Client, tx *types.Transaction, action string) error {
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return fmt.Errorf("等待%s交易确认失败: %w", action, err)
	}
	if receipt.Status == 0 {
		return fmt.Errorf("%s交易失败，交易状态为0", action)
	}
	log.Printf("%s交易确认成功，交易哈希: %s", action, tx.Hash().Hex())
	return nil
}

// approveTokenAndWait 授权路由合约使用代币，等待交易确认
func approveTokenAndWait(client *ethclient.Client, auth *bind.TransactOpts, token *bind.BoundContract, spender common.Address, amount *big.Int) error {
	var out []interface{}
	err := token.Call(&bind.CallOpts{Context: auth.Context}, &out, "allowance", auth.From, spender)
	if err != nil {
		return fmt.Errorf("查询allowance失败: %w", err)
	}
	if len(out) == 0 {
		return fmt.Errorf("查询allowance返回结果为空")
	}
	var allowance *big.Int
	switch v := out[0].(type) {
	case *big.Int:
		allowance = v
	case big.Int:
		allowance = new(big.Int).Set(&v)
	default:
		return fmt.Errorf("allowance返回类型不支持: %T", out[0])
	}
	if allowance.Cmp(amount) >= 0 {
		log.Println("已有足够授权，无需重复操作")
		return nil
	}

	// 授权极大值（避免重复授权）
	approveAmount := new(big.Int).Lsh(big.NewInt(1), 255) // 2^255

	tx, err := token.Transact(auth, "approve", spender, approveAmount)
	if err != nil {
		return fmt.Errorf("发送approve交易失败: %w", err)
	}
	log.Printf("授权交易已发送，哈希: %s", tx.Hash().Hex())

	if err := waitTransactionConfirmation(auth.Context, client, tx, "授权"); err != nil {
		return err
	}
	return nil
}

// findBestPath 自动筛选中间代币，返回最佳路径
func findBestPath(router *bind.BoundContract, from, to common.Address, amountIn *big.Int) ([]common.Address, error) {
	ctx := context.Background()

	var out []interface{}
	err := router.Call(&bind.CallOpts{Context: ctx}, &out, "getAmountsOut", amountIn, []common.Address{from, to})
	if err == nil {
		if amounts, err2 := extractBigIntSliceFromOut(out); err2 == nil {
			if len(amounts) >= 2 && amounts[len(amounts)-1].Cmp(big.NewInt(0)) > 0 {
				return []common.Address{from, to}, nil
			}
		}
	}

	bestAmountOut := big.NewInt(0)
	var bestPath []common.Address

	for _, mid := range candidateMiddleTokens {
		if mid == (common.Address{}) || mid == from || mid == to {
			continue
		}
		path := []common.Address{from, mid, to}
		out = nil
		err := router.Call(&bind.CallOpts{Context: ctx}, &out, "getAmountsOut", amountIn, path)
		if err != nil {
			continue
		}
		amounts, err2 := extractBigIntSliceFromOut(out)
		if err2 != nil {
			continue
		}
		if len(amounts) < 3 {
			continue
		}
		out := amounts[len(amounts)-1]
		if out.Cmp(bestAmountOut) > 0 {
			bestAmountOut = out
			bestPath = path
		}
	}

	if len(bestPath) > 0 {
		return bestPath, nil
	}

	return nil, fmt.Errorf("未找到有效兑换路径")
}

// swapExactETHForTokens 使用动态ABI调用swapExactETHForTokens购买代币
func swapExactETHForTokens(client *ethclient.Client, auth *bind.TransactOpts, router *bind.BoundContract, recipient common.Address, ethAmount *big.Int, path []common.Address, slippageBP int, deadlineSec int) error {
	ctx := context.Background()

	var out []interface{}
	err := router.Call(&bind.CallOpts{Context: ctx}, &out, "getAmountsOut", ethAmount, path)
	if err != nil {
		return fmt.Errorf("查询兑换数量失败: %w", err)
	}
	amounts, err2 := extractBigIntSliceFromOut(out)
	if err2 != nil {
		return fmt.Errorf("解析兑换数量失败: %w", err2)
	}
	if len(amounts) < 2 {
		return fmt.Errorf("兑换路径返回数量不足")
	}
	estimatedOut := amounts[len(amounts)-1]
	log.Printf("预计可获得目标代币数量: %s (Wei)", estimatedOut.String())

	amountOutMin := calculateSlippageAmount(estimatedOut, slippageBP)
	log.Printf("滑点(%d BP)后最小接收量: %s (Wei)", slippageBP, amountOutMin.String())

	deadline := big.NewInt(time.Now().Unix() + int64(deadlineSec))

	auth.Value = ethAmount
	auth.GasLimit = 0

	tx, err := router.Transact(auth, "swapExactETHForTokens", amountOutMin, path, recipient, deadline)
	if err != nil {
		return fmt.Errorf("调用swapExactETHForTokens失败: %w", err)
	}
	log.Printf("购买交易已发送，哈希: %s", tx.Hash().Hex())

	return waitTransactionConfirmation(ctx, client, tx, "购买")
}

// swapExactTokensForETH 使用动态ABI调用swapExactTokensForETH出售代币
func swapExactTokensForETH(client *ethclient.Client, auth *bind.TransactOpts, router *bind.BoundContract, recipient common.Address, tokenAmount *big.Int, path []common.Address, slippageBP int, deadlineSec int) error {
	ctx := context.Background()

	var out []interface{}
	err := router.Call(&bind.CallOpts{Context: ctx}, &out, "getAmountsOut", tokenAmount, path)
	if err != nil {
		return fmt.Errorf("查询兑换数量失败: %w", err)
	}
	amounts, err2 := extractBigIntSliceFromOut(out)
	if err2 != nil {
		return fmt.Errorf("解析兑换数量失败: %w", err2)
	}
	if len(amounts) < 2 {
		return fmt.Errorf("兑换路径返回数量不足")
	}
	estimatedOut := amounts[len(amounts)-1]
	log.Printf("预计可获得WNative数量: %s (Wei)", estimatedOut.String())

	amountOutMin := calculateSlippageAmount(estimatedOut, slippageBP)
	log.Printf("滑点(%d BP)后最小WNative接收量: %s (Wei)", slippageBP, amountOutMin.String())

	deadline := big.NewInt(time.Now().Unix() + int64(deadlineSec))

	auth.Value = big.NewInt(0)
	auth.GasLimit = 0

	tx, err := router.Transact(auth, "swapExactTokensForETH", tokenAmount, amountOutMin, path, recipient, deadline)
	if err != nil {
		return fmt.Errorf("调用swapExactTokensForETH失败: %w", err)
	}
	log.Printf("出售交易已发送，哈希: %s", tx.Hash().Hex())

	return waitTransactionConfirmation(ctx, client, tx, "出售")
}

// swapExactTokensForTokens 使用动态ABI调用代币对代币兑换
func swapExactTokensForTokens(client *ethclient.Client, auth *bind.TransactOpts, router *bind.BoundContract, recipient common.Address, tokenAmountIn *big.Int, path []common.Address, slippageBP int, deadlineSec int) error {
	ctx := context.Background()

	var out []interface{}
	err := router.Call(&bind.CallOpts{Context: ctx}, &out, "getAmountsOut", tokenAmountIn, path)
	if err != nil {
		return fmt.Errorf("查询兑换数量失败: %w", err)
	}
	amounts, err2 := extractBigIntSliceFromOut(out)
	if err2 != nil {
		return fmt.Errorf("解析兑换数量失败: %w", err2)
	}
	if len(amounts) < 2 {
		return fmt.Errorf("兑换路径返回数量不足")
	}
	estimatedOut := amounts[len(amounts)-1]
	log.Printf("预计可获得目标代币数量: %s (Wei)", estimatedOut.String())

	amountOutMin := calculateSlippageAmount(estimatedOut, slippageBP)
	log.Printf("滑点(%d BP)后最小接收量: %s (Wei)", slippageBP, amountOutMin.String())

	deadline := big.NewInt(time.Now().Unix() + int64(deadlineSec))

	auth.Value = big.NewInt(0)
	auth.GasLimit = 0

	tx, err := router.Transact(auth, "swapExactTokensForTokens", tokenAmountIn, amountOutMin, path, recipient, deadline)
	if err != nil {
		return fmt.Errorf("调用swapExactTokensForTokens失败: %w", err)
	}
	log.Printf("代币对代币交易已发送，哈希: %s", tx.Hash().Hex())

	return waitTransactionConfirmation(ctx, client, tx, "代币对代币")
}

// sendTransaction 发送交易函数（保留备用）
func sendTransaction(
	client *ethclient.Client,
	privateKey *ecdsa.PrivateKey,
	sender common.Address,
	to common.Address,
	value *big.Int,
	data []byte,
	defaultGasLimit uint64,
) error {
	nonce, err := client.PendingNonceAt(context.Background(), sender)
	if err != nil {
		return fmt.Errorf("获取Nonce失败: %w", err)
	}

	gasLimit, err := safeEstimateGas(
		client,
		ethereum.CallMsg{
			From:  sender,
			To:    &to,
			Value: value,
			Data:  data,
		},
		defaultGasLimit,
	)
	if err != nil {
		return fmt.Errorf("gas估算失败: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return fmt.Errorf("获取Gas价格失败: %w", err)
	}
	gasPrice = new(big.Int).Mul(gasPrice, big.NewInt(120))
	gasPrice.Div(gasPrice, big.NewInt(100))

	chainID := big.NewInt(xlayerConfig.ChainID)
	tx := types.NewTransaction(
		nonce,
		to,
		value,
		gasLimit,
		gasPrice,
		data,
	)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return fmt.Errorf("签名交易失败: %w", err)
	}

	if err := client.SendTransaction(context.Background(), signedTx); err != nil {
		return fmt.Errorf("发送交易失败: %w", err)
	}

	log.Printf("交易已发送，哈希: %s", signedTx.Hash().Hex())
	return nil
}

// safeEstimateGas 安全估算Gas函数（带缓冲和默认值）
func safeEstimateGas(client *ethclient.Client, msg ethereum.CallMsg, defaultGasLimit uint64) (uint64, error) {
	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		log.Printf("自动Gas估算失败: %v，将使用默认值: %d", err, defaultGasLimit)
		return defaultGasLimit, nil
	}

	bufferedGas := uint64(float64(gasLimit) * 1.2)
	log.Printf("Gas估算值: %d，添加20%%缓冲后: %d", gasLimit, bufferedGas)
	return bufferedGas, nil
}

// extractBigIntSliceFromOut 从通用返回结果中提取 []*big.Int
func extractBigIntSliceFromOut(out []interface{}) ([]*big.Int, error) {
	if len(out) == 0 {
		return nil, fmt.Errorf("结果为空")
	}
	switch v := out[0].(type) {
	case []*big.Int:
		return v, nil
	case []interface{}:
		result := make([]*big.Int, 0, len(v))
		for _, elem := range v {
			switch e := elem.(type) {
			case *big.Int:
				result = append(result, e)
			case big.Int:
				copyVal := new(big.Int).Set(&e)
				result = append(result, copyVal)
			default:
				return nil, fmt.Errorf("不支持的元素类型: %T", elem)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("不支持的返回类型: %T", out[0])
	}
}
