package model

import "testing"

func TestDefaultProvidersIncludesBuiltInProviders(t *testing.T) {
	want := []string{ProviderExa, ProviderYou, ProviderJina, ProviderTavily, ProviderFirecrawl, ProviderSerper, ProviderBrave}
	if len(DefaultProviders) != len(want) {
		t.Fatalf("DefaultProviders length = %d, want %d", len(DefaultProviders), len(want))
	}
	for index, provider := range want {
		if DefaultProviders[index] != provider {
			t.Fatalf("DefaultProviders[%d] = %q, want %q", index, DefaultProviders[index], provider)
		}
	}
}
