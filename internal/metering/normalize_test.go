package metering

import (
	"encoding/json"
	"testing"
)

func TestNormalizeRateTable(t *testing.T) {
	pricing := json.RawMessage(`{
		"base_rates_micro_per_unit": {"TOK_INPUT": 300, "TOK_OUTPUT": 900},
		"minimum_charge_micro": 100
	}`)
	raw := map[string]interface{}{
		"input_tokens":  1000,
		"output_tokens": 500,
	}
	micro, _, err := Normalize(pricing, raw, 10000)
	if err != nil {
		t.Fatal(err)
	}
	expected := 1000*300 + 500*900
	if micro != int64(expected) {
		t.Fatalf("got %d want %d", micro, expected)
	}
}

func TestNormalizeUSDPassthrough(t *testing.T) {
	pricing := json.RawMessage(`{
		"passthrough_usd": true,
		"platform_margin": 0.05,
		"minimum_charge_micro": 100
	}`)
	raw := map[string]interface{}{"cost_usd": 0.01}
	micro, _, err := Normalize(pricing, raw, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if micro < 100 {
		t.Fatalf("too low: %d", micro)
	}
}

func TestEstimateMax(t *testing.T) {
	pricing := json.RawMessage(`{"minimum_charge_micro": 100}`)
	got := EstimateMax(pricing, 5, 0)
	if got != 5_000_000 {
		t.Fatalf("got %d", got)
	}
}
