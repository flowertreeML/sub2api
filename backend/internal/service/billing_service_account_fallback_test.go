package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCalculateCostFromAccountFallback(t *testing.T) {
	inRate := 0.6
	outRate := 2.2
	zeroRate := 0.0

	tests := []struct {
		name          string
		account       *Account
		tokens        UsageTokens
		multiplier    float64
		inputCost     float64
		outputCost    float64
		cacheReadCost float64
		totalCost     float64
		actualCost    float64
	}{
		{
			name:       "input_only tokens",
			account:    &Account{CostPerMillionInput: &inRate, CostPerMillionOutput: &outRate},
			tokens:     UsageTokens{InputTokens: 1000},
			multiplier: 1,
			inputCost:  0.0006,
			totalCost:  0.0006,
			actualCost: 0.0006,
		},
		{
			name:       "output_only tokens",
			account:    &Account{CostPerMillionInput: &inRate, CostPerMillionOutput: &outRate},
			tokens:     UsageTokens{OutputTokens: 500},
			multiplier: 1,
			outputCost: 0.0011,
			totalCost:  0.0011,
			actualCost: 0.0011,
		},
		{
			name:       "input+output combined",
			account:    &Account{CostPerMillionInput: &inRate, CostPerMillionOutput: &outRate},
			tokens:     UsageTokens{InputTokens: 1000, OutputTokens: 500},
			multiplier: 1,
			inputCost:  0.0006,
			outputCost: 0.0011,
			totalCost:  0.0017,
			actualCost: 0.0017,
		},
		{
			name:          "cache_read tokens",
			account:       &Account{CostPerMillionInput: &inRate, CostPerMillionOutput: &outRate},
			tokens:        UsageTokens{CacheReadTokens: 2000},
			multiplier:    1,
			cacheReadCost: 0.0012,
			totalCost:     0.0012,
			actualCost:    0.0012,
		},
		{
			name:       "zero rate account",
			account:    &Account{CostPerMillionInput: &zeroRate, CostPerMillionOutput: &zeroRate},
			tokens:     UsageTokens{InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 2000},
			multiplier: 1,
		},
		{
			name:       "nil account",
			account:    nil,
			tokens:     UsageTokens{InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 2000},
			multiplier: 1,
		},
		{
			name:          "both input and cache_read together",
			account:       &Account{CostPerMillionInput: &inRate, CostPerMillionOutput: &outRate},
			tokens:        UsageTokens{InputTokens: 1000, CacheReadTokens: 2000},
			multiplier:    1,
			inputCost:     0.0006,
			cacheReadCost: 0.0012,
			totalCost:     0.0018,
			actualCost:    0.0018,
		},
		{
			name:       "multiplier 0.5",
			account:    &Account{CostPerMillionInput: &inRate, CostPerMillionOutput: &outRate},
			tokens:     UsageTokens{InputTokens: 1000, OutputTokens: 500},
			multiplier: 0.5,
			inputCost:  0.0003,
			outputCost: 0.00055,
			totalCost:  0.00085,
			actualCost: 0.00085,
		},
		{
			name:       "negative multiplier clamps to zero",
			account:    &Account{CostPerMillionInput: &inRate, CostPerMillionOutput: &outRate},
			tokens:     UsageTokens{InputTokens: 1000, OutputTokens: 500},
			multiplier: -1,
		},
		{
			name:       "input field nil",
			account:    &Account{CostPerMillionOutput: &outRate},
			tokens:     UsageTokens{InputTokens: 1000, OutputTokens: 500},
			multiplier: 1,
		},
		{
			name:       "output field nil",
			account:    &Account{CostPerMillionInput: &inRate},
			tokens:     UsageTokens{InputTokens: 1000, OutputTokens: 500},
			multiplier: 1,
		},
	}

	svc := NewBillingService(&config.Config{}, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := svc.CalculateCostFromAccountFallback(tt.account, tt.tokens, tt.multiplier)
			require.NotNil(t, cost)
			require.InDelta(t, tt.inputCost, cost.InputCost, 1e-9)
			require.InDelta(t, tt.outputCost, cost.OutputCost, 1e-9)
			require.InDelta(t, tt.cacheReadCost, cost.CacheReadCost, 1e-9)
			require.InDelta(t, tt.totalCost, cost.TotalCost, 1e-9)
			require.InDelta(t, tt.actualCost, cost.ActualCost, 1e-9)
			if tt.actualCost > 0 {
				require.Equal(t, string(BillingModeToken), cost.BillingMode)
			}
		})
	}
}
