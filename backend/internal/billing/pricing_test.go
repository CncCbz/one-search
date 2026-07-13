package billing

import "testing"

func TestEstimateCostUSD(t *testing.T) {
	cost, ok := EstimateCostUSD("exa", "requests", 1000)
	if !ok || cost < 6.9 || cost > 7.1 {
		t.Fatalf("exa 1000 req => %v ok=%v", cost, ok)
	}
	cost, ok = EstimateCostUSD("tavily", "credits", 10)
	if !ok || cost < 0.07 || cost > 0.09 {
		t.Fatalf("tavily 10 credits => %v ok=%v", cost, ok)
	}
	if _, ok := EstimateCostUSD("unknown", "requests", 1); ok {
		t.Fatal("unknown provider should not estimate")
	}
}

func TestRateFromSettingsOverride(t *testing.T) {
	rate := RateFromSettings("exa", map[string]interface{}{"price_per_request": 0.01})
	cost, ok := EstimateCostUSDWithRate(rate, "requests", 100)
	if !ok || cost < 0.99 || cost > 1.01 {
		t.Fatalf("custom exa rate => %v ok=%v", cost, ok)
	}
	rate = RateFromSettings("tavily", map[string]interface{}{"default_billable_credits": 3})
	if rate.DefaultBillableCredit != 3 {
		t.Fatalf("default credits override => %v", rate.DefaultBillableCredit)
	}
}
