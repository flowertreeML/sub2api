package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCalculateRecordUsageCost_UsesAccountCostPerMillionBeforeZhipuDefault(t *testing.T) {
	inputPrice := 4.0
	outputPrice := 16.0
	svc := &GatewayService{billingService: NewBillingService(&config.Config{}, nil)}

	cost := svc.calculateRecordUsageCost(context.Background(), &ForwardResult{
		Usage: ClaudeUsage{InputTokens: 1_000_000, OutputTokens: 1_000_000},
	}, &APIKey{}, &Account{
		Platform:                 PlatformZhipu,
		CostPerMillionInput:      &inputPrice,
		CostPerMillionOutput:     &outputPrice,
	}, "glm-4.6", 1.0, &recordUsageOpts{})

	require.NotNil(t, cost)
	require.InDelta(t, 4.0, cost.InputCost, 0.000001)
	require.InDelta(t, 16.0, cost.OutputCost, 0.000001)
	require.InDelta(t, 20.0, cost.ActualCost, 0.000001)
}

func TestCalculateRecordUsageCost_UsesZhipuDefaultWhenAccountCostMissing(t *testing.T) {
	svc := &GatewayService{billingService: NewBillingService(&config.Config{}, nil)}

	cost := svc.calculateRecordUsageCost(context.Background(), &ForwardResult{
		Usage: ClaudeUsage{InputTokens: 1_000_000, OutputTokens: 1_000_000},
	}, &APIKey{}, &Account{Platform: PlatformZhipu}, "glm-4.6", 1.0, &recordUsageOpts{})

	require.NotNil(t, cost)
	require.InDelta(t, 4.0, cost.InputCost, 0.000001)
	require.InDelta(t, 16.0, cost.OutputCost, 0.000001)
	require.InDelta(t, 20.0, cost.ActualCost, 0.000001)
}

func TestCalculateRecordUsageCost_UnknownZhipuModelFallsThroughToZero(t *testing.T) {
	svc := &GatewayService{billingService: NewBillingService(&config.Config{}, nil)}

	cost := svc.calculateRecordUsageCost(context.Background(), &ForwardResult{
		Usage: ClaudeUsage{InputTokens: 1_000_000, OutputTokens: 1_000_000},
	}, &APIKey{}, &Account{Platform: PlatformZhipu}, "totally-unknown-glm", 1.0, &recordUsageOpts{})

	require.NotNil(t, cost)
	require.Zero(t, cost.ActualCost)
}
