package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"log"
	"math/big"
	"os"
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
}{
	TokenAddr:  "0x0cc24c51bf89c00c5affbfcf5e856c25ecbdb48e", // XDOG代币地址
	OkbAddr:    OKBAddress,                                   // okb地址
	RouterAddr: RouterAddress,                                // router地址
	ChainID:    196,                                          // x layer
	SlippageBP: 50,                                           // 设置的滑点
	Deadline:   30,                                           // 超时时间30s
	PrivateKey: "private_key",                                // okx申请的私钥
}

func main() {
	// 解析命令行参数
	if len(os.Args) < 3 {
		fmt.Printf("用法: %s <sell|buy> <数量> [代币地址]\n", os.Args[0])
		os.Exit(1)
	}

	opType := strings.ToLower(os.Args[1])
	amountStr := os.Args[2]
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		log.Fatalf("数量解析错误: %v", err)
	}
	tokenAddr := cfg.TokenAddr
	if len(os.Args) > 3 {
		tokenAddr = os.Args[3]
	}

	if opType != "sell" && opType != "buy" {
		log.Fatalf("不支持的操作类型: %s (只能是 sell 或 buy)", opType)
	}

	// 连接 RPC 节点
	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, RpcUrl)
	if err != nil {
		log.Fatal("连接 RPC 失败:", err)
	}
	fmt.Println("已连接到 X Layer 节点")

	// 打印操作信息
	fmt.Printf("操作: %s\n数量: %f\n代币地址: %s\n", opType, amount, tokenAddr)

	// 根据操作类型选择路径
	tokenAddrCommon := common.HexToAddress(tokenAddr)
	okbAddrCommon := common.HexToAddress(cfg.OkbAddr)

	var tokenIn, tokenOut common.Address
	if opType == "sell" {
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
		TokenAddr:  common.HexToAddress(tokenAddr),
		OkbAddr:    common.HexToAddress(cfg.OkbAddr),
		RouterAddr: common.HexToAddress(cfg.RouterAddr),
		ChainID:    big.NewInt(cfg.ChainID),
		SlippageBP: cfg.SlippageBP,
		Deadline:   time.Duration(cfg.Deadline) * time.Second,
	}

	// 调用 SwapTokens
	err = SwapTokens(ctx, client, sellCfg, tokenIn, tokenOut, amount)
	if err != nil {
		log.Fatalf("交易执行失败: %v", err)
	}
	fmt.Println("交易执行完成。")
}
