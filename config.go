package main

const pairABI = `[{"constant":true,"inputs":[],"name":"getReserves","outputs":[{"internalType":"uint112","name":"_reserve0","type":"uint112"},{"internalType":"uint112","name":"_reserve1","type":"uint112"},{"internalType":"uint32","name":"_blockTimestampLast","type":"uint32"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"token0","outputs":[{"internalType":"address","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"token1","outputs":[{"internalType":"address","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"}]`
const erc20ABI = `[{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}]`

// RpcUrl 主网 RPC
const RpcUrl = "https://xlayerrpc.okx.com"

// PrivateKeyHex 你的测试私钥
const PrivateKeyHex = "你的私钥"

// WalletAddress 钱包地址
const WalletAddress = "0xYourAddress"

// RouterAddress dex router地址 UniswapV2
const RouterAddress = "0xYourRouterAddress"
