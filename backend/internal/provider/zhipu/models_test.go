package zhipu

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestDefaultBaseURL(t *testing.T) {
	want := "https://open.bigmodel.cn/api/paas/v4"
	if DefaultBaseURL != want {
		t.Errorf("DefaultBaseURL = %q; want %q", DefaultBaseURL, want)
	}
}

func TestPlatformValue(t *testing.T) {
	if PlatformValue != "zhipu" {
		t.Errorf("PlatformValue = %q; want \"zhipu\"", PlatformValue)
	}
}

func TestLookupModel_Glm46(t *testing.T) {
	spec, ok := LookupModel("glm-4.6")
	if !ok {
		t.Fatalf("not found")
	}
	if !spec.InputPricePer1M.Equal(decimal.NewFromInt(4)) || !spec.OutputPricePer1M.Equal(decimal.NewFromInt(16)) {
		t.Errorf("glm-4.6 pricing = (%s, %s); want (4, 16)",
			spec.InputPricePer1M.String(), spec.OutputPricePer1M.String())
	}
}

func TestLookupModel_Unknown(t *testing.T) {
	if _, ok := LookupModel("totally-unknown-model"); ok {
		t.Error("expected ok=false for unknown model")
	}
}

func TestSupportedModels_HasFlash(t *testing.T) {
	if _, ok := SupportedModels["glm-4.5-flash"]; ok {
		return
	}
	t.Error("missing glm-4.5-flash")
}
