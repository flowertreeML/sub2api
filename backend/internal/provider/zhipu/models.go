package zhipu

import "github.com/shopspring/decimal"

const (
	// DefaultBaseURL 是智谱 BigModel OpenAI 兼容接口的默认根地址。
	DefaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"
	// PlatformValue 是系统内智谱渠道的平台标识。
	PlatformValue = "zhipu"
)

// ModelSpec 描述智谱模型的默认元数据和每百万 Token 单价。
type ModelSpec struct {
	Name              string
	InputPricePer1M   decimal.Decimal
	OutputPricePer1M  decimal.Decimal
}

// SupportedModels 保存智谱官方公开单价；单位为人民币元/百万 Tokens。
var SupportedModels = map[string]ModelSpec{
	"glm-4.6":              newModelSpec("glm-4.6", "4", "16"),
	"glm-4.5":              newModelSpec("glm-4.5", "2", "8"),
	"glm-4.5-x":            newModelSpec("glm-4.5-x", "2.2", "8.9"),
	"glm-4.5-air":          newModelSpec("glm-4.5-air", "0.5", "0.5"),
	"glm-4.5-airx":         newModelSpec("glm-4.5-airx", "10", "10"),
	"glm-4.5-flash":        newModelSpec("glm-4.5-flash", "0", "0"),
	"glm-4":                newModelSpec("glm-4", "5", "5"),
	"glm-4-plus":           newModelSpec("glm-4-plus", "5", "5"),
	"glm-4-air":            newModelSpec("glm-4-air", "0.5", "0.5"),
	"glm-4-air-250414":     newModelSpec("glm-4-air-250414", "0.5", "0.5"),
	"glm-4-airx":           newModelSpec("glm-4-airx", "10", "10"),
	"glm-4-flash":          newModelSpec("glm-4-flash", "0", "0"),
	"glm-4-flash-250414":   newModelSpec("glm-4-flash-250414", "0", "0"),
	"glm-4-flashx":         newModelSpec("glm-4-flashx", "0.1", "0.1"),
	"glm-4-flashx-250414":  newModelSpec("glm-4-flashx-250414", "0.1", "0.1"),
	"glm-4-long":           newModelSpec("glm-4-long", "1", "1"),
	"glm-4v":               newModelSpec("glm-4v", "0", "0"),
	"glm-4v-flash":         newModelSpec("glm-4v-flash", "0", "0"),
	"glm-4v-plus":          newModelSpec("glm-4v-plus", "4", "4"),
	"glm-4v-plus-0111":     newModelSpec("glm-4v-plus-0111", "4", "4"),
	"glm-z1-air":           newModelSpec("glm-z1-air", "0.5", "0.5"),
	"glm-z1-airx":          newModelSpec("glm-z1-airx", "5", "5"),
	"glm-z1-flash":         newModelSpec("glm-z1-flash", "0", "0"),
	"glm-z1-flashx":        newModelSpec("glm-z1-flashx", "0.1", "0.1"),
}

// LookupModel 按模型名查找智谱默认单价。
func LookupModel(name string) (ModelSpec, bool) {
	spec, ok := SupportedModels[name]
	return spec, ok
}

func newModelSpec(name, input, output string) ModelSpec {
	return ModelSpec{
		Name:             name,
		InputPricePer1M:  decimal.RequireFromString(input),
		OutputPricePer1M: decimal.RequireFromString(output),
	}
}
