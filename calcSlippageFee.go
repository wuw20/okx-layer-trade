package main

import "math/big"

// CalcAmountOutMin 计算滑点
func CalcAmountOutMin(expectedOut *big.Int, slippage float64) *big.Int {
	expectedOutFloat := new(big.Float).SetInt(expectedOut)
	oneMinusSlippage := big.NewFloat(1 - slippage)
	amountOutMinFloat := new(big.Float).Mul(expectedOutFloat, oneMinusSlippage)
	amountOutMin := new(big.Int)
	amountOutMinFloat.Int(amountOutMin)
	return amountOutMin
}
