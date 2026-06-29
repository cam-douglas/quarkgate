package metering

import (
	"encoding/json"
	"math"
)

// PricingModel mirrors provider_configs.pricing_model JSON.
type PricingModel struct {
	PassthroughUSD       bool              `json:"passthrough_usd"`
	PlatformMargin       float64           `json:"platform_margin"`
	BaseRatesMicroPerUnit map[string]int64 `json:"base_rates_micro_per_unit"`
	ModelOverrides       map[string]map[string]int64 `json:"model_overrides"`
	MinimumChargeMicro   int64             `json:"minimum_charge_micro"`
}

// NormalizeOptions tunes normalization for edge cases.
type NormalizeOptions struct {
	PlatformMargin float64
	Partial        bool
}

// Normalize converts raw usage metrics to micro_credits.
func Normalize(pricing json.RawMessage, raw map[string]interface{}, creditUSDMicro int64) (int64, json.RawMessage, error) {
	return NormalizeWithOptions(pricing, raw, creditUSDMicro, NormalizeOptions{})
}

func NormalizeWithOptions(pricing json.RawMessage, raw map[string]interface{}, creditUSDMicro int64, opts NormalizeOptions) (int64, json.RawMessage, error) {
	var pm PricingModel
	if err := json.Unmarshal(pricing, &pm); err != nil {
		return 0, nil, err
	}

	if opts.PlatformMargin > 0 {
		pm.PlatformMargin = opts.PlatformMargin
	}
	if pm.PlatformMargin == 0 {
		pm.PlatformMargin = 0.05
	}

	// USD passthrough path (OpenRouter)
	if pm.PassthroughUSD {
		if costUSD, ok := raw["cost_usd"].(float64); ok && costUSD > 0 {
			usdMicro := int64(math.Round(costUSD * 1_000_000))
			micro := int64(math.Round(float64(usdMicro) * (1.0 + pm.PlatformMargin) * float64(creditUSDMicro) / 1_000_000))
			if micro < pm.MinimumChargeMicro {
				micro = pm.MinimumChargeMicro
			}
			norm := map[string]interface{}{
				"micro_credits": micro,
				"cost_usd":      costUSD,
				"method":        "passthrough_usd",
			}
			b, _ := json.Marshal(norm)
			return micro, b, nil
		}
	}

	var total int64
	quantities := map[string]float64{}

	if v, ok := raw["input_tokens"].(float64); ok {
		quantities["TOK_INPUT"] = v
	}
	if v, ok := raw["input_tokens"].(int); ok {
		quantities["TOK_INPUT"] = float64(v)
	}
	if v, ok := raw["output_tokens"].(float64); ok {
		quantities["TOK_OUTPUT"] = v
	}
	if v, ok := raw["output_tokens"].(int); ok {
		quantities["TOK_OUTPUT"] = float64(v)
	}
	if v, ok := raw["compute_seconds"].(float64); ok {
		quantities["COMPUTE_S"] = v
	}
	if v, ok := raw["compute_seconds"].(int); ok {
		quantities["COMPUTE_S"] = float64(v)
	}
	if v, ok := raw["db_reads"].(float64); ok {
		quantities["DB_READ"] = v
	}
	if v, ok := raw["db_reads"].(int); ok {
		quantities["DB_READ"] = float64(v)
	}
	if v, ok := raw["db_writes"].(float64); ok {
		quantities["DB_WRITE"] = v
	}
	if v, ok := raw["db_writes"].(int); ok {
		quantities["DB_WRITE"] = float64(v)
	}
	if v, ok := raw["vec_queries"].(float64); ok {
		quantities["VEC_QUERY"] = v
	}
	if v, ok := raw["vec_queries"].(int); ok {
		quantities["VEC_QUERY"] = float64(v)
	}
	if v, ok := raw["api_calls"].(float64); ok {
		quantities["API_CALL"] = v
	}
	if v, ok := raw["api_calls"].(int); ok {
		quantities["API_CALL"] = float64(v)
	}

	model, _ := raw["model"].(string)
	rates := pm.BaseRatesMicroPerUnit
	if model != "" && pm.ModelOverrides != nil {
		if override, ok := pm.ModelOverrides[model]; ok {
			for k, v := range override {
				rates[k] = v
			}
		}
	}

	for unit, qty := range quantities {
		if rate, ok := rates[unit]; ok {
			total += int64(math.Round(qty * float64(rate)))
		}
	}

	if total < pm.MinimumChargeMicro && total > 0 {
		total = pm.MinimumChargeMicro
	}
	if total == 0 && pm.MinimumChargeMicro > 0 && len(quantities) > 0 {
		total = pm.MinimumChargeMicro
	}
	if opts.Partial && total > 0 && pm.MinimumChargeMicro > 0 && total < pm.MinimumChargeMicro {
		total = pm.MinimumChargeMicro
	}

	norm := map[string]interface{}{
		"micro_credits": total,
		"quantities":    quantities,
		"method":        "rate_table",
	}
	b, _ := json.Marshal(norm)
	return total, b, nil
}

func EstimateMax(pricing json.RawMessage, hints int64, defaultEstimate int64) int64 {
	if hints > 0 {
		return hints * 1_000_000
	}
	var pm PricingModel
	if err := json.Unmarshal(pricing, &pm); err == nil && pm.MinimumChargeMicro > 0 {
		if defaultEstimate < pm.MinimumChargeMicro {
			return pm.MinimumChargeMicro * 100
		}
	}
	if defaultEstimate == 0 {
		return 10_000_000 // 10 credits default hold
	}
	return defaultEstimate
}
