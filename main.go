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

// DEXRouterABI 包含OKB相关交易方法的ABI
const DEXRouterABI = `[
	{
		"constant": false,
		"inputs": [
			{"name": "amountIn", "type": "uint256"},
			{"name": "amountOutMin", "type": "uint256"},
			{"name": "path", "type": "address[]"},
			{"name": "to", "type": "address"},
			{"name": "deadline", "type": "uint256"}
		],
		"name": "swapExactTokensForTokens",
		"outputs": [{"name": "amounts", "type": "uint256[]"}],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

// ERC20ABI 用于代币授权的ABI
const ERC20ABI = `[
	{
		"constant": false,
		"inputs": [
			{"name": "spender", "type": "address"},
			{"name": "amount", "type": "uint256"}
		],
		"name": "approve",
		"outputs": [{"name": "", "type": "bool"}],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{"name": "owner", "type": "address"},
			{"name": "spender", "type": "address"}
		],
		"name": "allowance",
		"outputs": [{"name": "", "type": "uint256"}],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "decimals",
		"outputs": [{"name": "", "type": "uint8"}],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}
]`

// XLayer网络配置
var xlayerConfig = struct {
	RPCEndpoint   string
	RouterAddress string
	TokenAddr     string
	OKBAddress    string
	RouterAddr    string
	ChainID       int64
	SlippageBP    int
	Deadline      int
	PrivateKey    string
	opType        string
	amount        string
	AutoApprove   bool
}{
	RPCEndpoint:   "https://xlayerrpc.okx.com",
	RouterAddress: "0x127a986cE31AA2ea8E1a6a0F0D5b7E5dbaD7b0bE", // 替换为XLayer上DEX的Router地址
	TokenAddr:     "0x0cc24c51bf89c00c5affbfcf5e856c25ecbdb48e", // 代币地址
	OKBAddress:    "0xe538905cf8410324e03a5a23c1c177a474d59b2b", // okb地址
	RouterAddr:    "0x127a986cE31AA2ea8E1a6a0F0D5b7E5dbaD7b0bE", // xlayer 部署节点地址
	ChainID:       196,
	SlippageBP:    5,
	Deadline:      30,
	PrivateKey:    "0x623f230c83b4343cd0e2423a6114ca4e8a16b57f596b9ab229cbc1d9e077b7fc", // okx生产的一个apiKey
	opType:        "buy",
	amount:        "0.001",
	AutoApprove:   false,
}

func main() {
	// 连接到XLayer节点
	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, xlayerConfig.RPCEndpoint)
	if err != nil {
		log.Fatalf("无法连接到XLayer节点: %v", err)
	}
	defer client.Close()

	// 加载私钥（实际使用中请安全存储）
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(xlayerConfig.PrivateKey, "0x"))
	if err != nil {
		log.Fatalf("私钥解析错误: %v", err)
	}

	// 获取发送者地址
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	senderAddress := crypto.PubkeyToAddress(*publicKey)
	fmt.Printf("使用地址: %s\n", senderAddress.Hex())

	tokenAddress := common.HexToAddress(xlayerConfig.TokenAddr) // 要交易的代币
	amount, err := strconv.ParseFloat(xlayerConfig.amount, 64)
	if err != nil {
		log.Fatalf("金额解析失败: %v", err)
	}

	erc20ABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	decimals, err := getTokenDecimals(client, tokenAddress, erc20ABI)
	if err != nil || decimals == 0 {
		fmt.Printf("代币精度解析失败: %v\n", err)
	}
	tradeAmount := ToWeiFloat(amount, decimals)
	slippage := xlayerConfig.SlippageBP

	// 示例1: 用OKB购买代币
	if xlayerConfig.opType == "buy" {
		// 先授权Router使用OKB
		err = approveToken(client, privateKey, senderAddress, xlayerConfig.OKBAddress, xlayerConfig.RouterAddress, tradeAmount)
		if err != nil {
			log.Fatalf("授权OKB失败: %v", err)
		}

		// 用OKB购买代币
		err = swapOKBForTokens(client, privateKey, senderAddress, tradeAmount, tokenAddress, slippage)
		if err != nil {
			log.Fatalf("用OKB购买代币失败: %v", err)
		}
	} else if xlayerConfig.opType == "sell" {
		// 示例2: 用代币兑换OKB
		// 先授权Router使用要出售的代币
		err = approveToken(client, privateKey, senderAddress, tokenAddress.Hex(), xlayerConfig.RouterAddress, tradeAmount)
		if err != nil {
			log.Fatalf("授权代币失败: %v", err)
		}

		// 用代币兑换OKB
		err = swapTokensForOKB(client, privateKey, senderAddress, tradeAmount, tokenAddress, slippage)
		if err != nil {
			log.Fatalf("用代币兑换OKB失败: %v", err)
		}
	} else {
		fmt.Println("暂不支持的交易类型")
	}
}

// 获取代币的小数位数
func getTokenDecimals(client *ethclient.Client, tokenAddress common.Address, erc20ABI abi.ABI) (int, error) {
	// 编码调用数据
	data, err := erc20ABI.Pack("decimals")
	if err != nil {
		return 0, fmt.Errorf("编码decimals调用数据失败: %w", err)
	}

	// 调用合约
	result, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &tokenAddress,
		Data: data,
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("调用decimals失败: %w", err)
	}

	// 解析结果
	var decimals uint8
	if err := erc20ABI.UnpackIntoInterface(&decimals, "decimals", result); err != nil {
		return 0, fmt.Errorf("解析decimals结果失败: %w", err)
	}

	return int(decimals), nil
}

func ToWeiFloat(amount float64, decimals int) *big.Int {
	base := new(big.Float).SetFloat64(math.Pow10(decimals))
	value := new(big.Float).Mul(big.NewFloat(amount), base)
	result := new(big.Int)
	value.Int(result)
	return result
}

// swapOKBForTokens 用OKB购买其他代币
func swapOKBForTokens(client *ethclient.Client, privateKey *ecdsa.PrivateKey, sender common.Address,
	okbAmount *big.Int, tokenAddress common.Address, slippagePercent int) error {

	// 1. 准备交易路径: OKB -> 目标代币
	routerAddr := common.HexToAddress(xlayerConfig.RouterAddress)
	okbAddr := common.HexToAddress(xlayerConfig.OKBAddress)
	path := []common.Address{okbAddr, tokenAddress}
	deadline := big.NewInt(time.Now().Unix() + 600) // 10分钟后过期

	// 2. 估算输出代币数量并计算最小接收量
	// 实际应用中应先调用router的getAmountsOut方法获取精确值
	estimatedOut := estimateTokenAmount(okbAmount)
	if estimatedOut.Cmp(big.NewInt(0)) <= 0 {
		return fmt.Errorf("无法估算输出代币数量")
	}

	// 计算滑点后的最小接收量
	slippage := new(big.Int).Mul(estimatedOut, big.NewInt(int64(slippagePercent)))
	slippage.Div(slippage, big.NewInt(100))
	amountOutMin := new(big.Int).Sub(estimatedOut, slippage)

	// 3. 编码交易数据
	routerABI, err := abi.JSON(strings.NewReader(DEXRouterABI))
	if err != nil {
		return fmt.Errorf("解析ABI失败: %w", err)
	}

	data, err := routerABI.Pack("swapExactTokensForTokens",
		okbAmount,
		amountOutMin,
		path,
		sender,
		deadline,
	)
	if err != nil {
		return fmt.Errorf("编码交易数据失败: %w", err)
	}

	// 4. 发送交易（注意：用代币交易时value为0）
	return sendTransaction(client, privateKey, sender, routerAddr, big.NewInt(5000), data)
}

// swapTokensForOKB 用其他代币兑换OKB
func swapTokensForOKB(client *ethclient.Client, privateKey *ecdsa.PrivateKey, sender common.Address,
	tokenAmount *big.Int, tokenAddress common.Address, slippagePercent int) error {

	// 1. 准备交易路径: 代币 -> OKB
	routerAddr := common.HexToAddress(xlayerConfig.RouterAddress)
	okbAddr := common.HexToAddress(xlayerConfig.OKBAddress)
	path := []common.Address{tokenAddress, okbAddr}
	deadline := big.NewInt(time.Now().Unix() + 600)

	// 2. 估算输出OKB数量并计算最小接收量
	estimatedOut := estimateTokenAmount(tokenAmount)
	if estimatedOut.Cmp(big.NewInt(0)) <= 0 {
		return fmt.Errorf("无法估算输出OKB数量")
	}

	// 计算滑点后的最小接收量
	slippage := new(big.Int).Mul(estimatedOut, big.NewInt(int64(slippagePercent)))
	slippage.Div(slippage, big.NewInt(100))
	amountOutMin := new(big.Int).Sub(estimatedOut, slippage)

	// 3. 编码交易数据
	routerABI, err := abi.JSON(strings.NewReader(DEXRouterABI))
	if err != nil {
		return fmt.Errorf("解析ABI失败: %w", err)
	}

	data, err := routerABI.Pack("swapExactTokensForTokens",
		tokenAmount,
		amountOutMin,
		path,
		sender,
		deadline,
	)
	if err != nil {
		return fmt.Errorf("编码交易数据失败: %w", err)
	}

	// 4. 发送交易
	return sendTransaction(client, privateKey, sender, routerAddr, big.NewInt(0), data)
}

// approveToken 授权Router合约使用指定代币
func approveToken(client *ethclient.Client, privateKey *ecdsa.PrivateKey, sender common.Address,
	tokenAddress string, spenderAddress string, amount *big.Int) error {

	tokenAddr := common.HexToAddress(tokenAddress)
	spenderAddr := common.HexToAddress(spenderAddress)

	// 检查当前授权额度
	erc20ABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return fmt.Errorf("解析ERC20 ABI失败: %w", err)
	}

	callData, err := erc20ABI.Pack("allowance", sender, spenderAddr)
	if err != nil {
		return fmt.Errorf("编码allowance调用数据失败: %w", err)
	}

	result, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &tokenAddr,
		Data: callData,
	}, nil)
	if err != nil {
		return fmt.Errorf("调用allowance失败: %w", err)
	}

	var allowance *big.Int
	if err := erc20ABI.UnpackIntoInterface(&allowance, "allowance", result); err != nil {
		return fmt.Errorf("解析allowance结果失败: %w", err)
	}

	// 如果已有足够授权，无需重复授权
	if allowance.Cmp(amount) >= 0 {
		fmt.Println("已有足够授权，无需重复操作")
		return nil
	}

	// 编码授权交易数据
	data, err := erc20ABI.Pack("approve", spenderAddr, amount)
	if err != nil {
		return fmt.Errorf("编码approve数据失败: %w", err)
	}

	// 发送授权交易
	return sendTransaction(client, privateKey, sender, tokenAddr, big.NewInt(0), data)
}

// estimateTokenAmount 估算交易输出数量（简化实现）
func estimateTokenAmount(amountIn *big.Int) *big.Int {
	// 实际应用中应调用router的getAmountsOut方法
	// 这里简化处理，返回一个假设值
	return new(big.Int).Mul(amountIn, big.NewInt(10)) // 假设1:10的兑换比例
}

// sendTransaction 通用交易发送函数
func sendTransaction(client *ethclient.Client, privateKey *ecdsa.PrivateKey,
	sender common.Address, to common.Address, value *big.Int, data []byte) error {

	// 获取nonce
	nonce, err := client.PendingNonceAt(context.Background(), sender)
	if err != nil {
		return fmt.Errorf("获取nonce失败: %w", err)
	}

	// 估算gas 预估最小值
	gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{
		From:  sender,
		To:    &to,
		Value: value,
		Data:  data,
	})
	if err != nil {
		return fmt.Errorf("估算gas失败: %w", err)
	}

	// 获取gas价格
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return fmt.Errorf("获取gas价格失败: %w", err)
	}

	// 创建交易
	chainID := big.NewInt(xlayerConfig.ChainID)
	tx := types.NewTransaction(nonce, to, value, gasLimit, gasPrice, data)

	// 签名交易
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return fmt.Errorf("签名交易失败: %w", err)
	}

	// 发送交易
	if err := client.SendTransaction(context.Background(), signedTx); err != nil {
		return fmt.Errorf("发送交易失败: %w", err)
	}

	fmt.Printf("交易已发送: %s\n", signedTx.Hash().Hex())
	return nil
}
