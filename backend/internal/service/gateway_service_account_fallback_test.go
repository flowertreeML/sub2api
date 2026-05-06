package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCalculateTokenCost_AccountFallbackWhenPricingMissing(t *testing.T) {
	inRate := 0.6
	outRate := 2.2
	svc := &GatewayService{
		billingService: NewBillingService(&config.Config{}, nil),
	}

	cost := svc.calculateTokenCost(
		context.Background(),
		&ForwardResult{
			Usage: ClaudeUsage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
		},
		&APIKey{},
		&Account{ID: 42, CostPerMillionInput: &inRate, CostPerMillionOutput: &outRate},
		"__test_unknown_model__",
		1,
		&recordUsageOpts{},
	)

	require.NotNil(t, cost)
	require.Greater(t, cost.ActualCost, 0.0)
	require.InDelta(t, 0.0017, cost.ActualCost, 1e-9)
}

func TestApplyAccountCostFallbackIfPricingNotFound_WrappedSentinel(t *testing.T) {
	inRate := 0.6
	outRate := 2.2
	svc := &GatewayService{
		billingService: NewBillingService(&config.Config{}, nil),
	}

	cost, err := svc.applyAccountCostFallbackIfPricingNotFound(
		fmt.Errorf("out-range cost: %w", ErrPricingNotFound),
		&Account{ID: 42, CostPerMillionInput: &inRate, CostPerMillionOutput: &outRate},
		UsageTokens{InputTokens: 1000, OutputTokens: 500},
		1,
		"__test_unknown_model__",
		nil,
	)

	require.NoError(t, err)
	require.NotNil(t, cost)
	require.Greater(t, cost.ActualCost, 0.0)
	require.InDelta(t, 0.0017, cost.ActualCost, 1e-9)
}

func TestApplyAccountCostFallbackIfPricingNotFound_NonPricingErrorDoesNotTriggerFallback(t *testing.T) {
	inRate := 0.6
	outRate := 2.2
	originalErr := fmt.Errorf("network error")
	svc := &GatewayService{
		billingService: NewBillingService(&config.Config{}, nil),
	}

	cost, err := svc.applyAccountCostFallbackIfPricingNotFound(
		originalErr,
		&Account{ID: 42, CostPerMillionInput: &inRate, CostPerMillionOutput: &outRate},
		UsageTokens{InputTokens: 1000, OutputTokens: 500},
		1,
		"__test_unknown_model__",
		&CostBreakdown{ActualCost: 0},
	)

	require.ErrorIs(t, err, originalErr)
	require.NotNil(t, cost)
	require.InDelta(t, 0.0, cost.ActualCost, 1e-9)
}
