package bill

import (
	"testing"

	"github.com/fisker086/keyops/pkg/config"
)

func TestNormalizeDisplayCurrency(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"USD", "USD"},
		{"usd", "USD"},
		{"CNY", "CNY"},
		{"cny", "CNY"},
		{"", "CNY"},
		{"eur", "CNY"},
	}
	for _, tc := range tests {
		if got := NormalizeDisplayCurrency(tc.in); got != tc.want {
			t.Fatalf("NormalizeDisplayCurrency(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestVendorNativeToUSD(t *testing.T) {
	const rate = 6.8
	s := &BillService{usdToCNYRate: rate}
	if got := s.vendorNativeToUSD("aws", 522490); got != 522490 {
		t.Fatalf("aws usd got %v", got)
	}
	if got := s.vendorNativeToUSD("aliyun", 6800); got != 1000 {
		t.Fatalf("aliyun usd got %v want 1000", got)
	}
}

func TestUsdToDisplayCurrency(t *testing.T) {
	const rate = 6.8
	s := &BillService{usdToCNYRate: rate}
	const usd = 522490.0

	if got := s.usdToDisplayCurrency("USD", usd); got != usd {
		t.Fatalf("USD display got %v want %v", got, usd)
	}
	wantCNY := usd * rate
	if got := s.usdToDisplayCurrency("CNY", usd); got != wantCNY {
		t.Fatalf("CNY display got %v want %v", got, wantCNY)
	}
	if got := s.usdToDisplayCurrency("cny", usd); got != wantCNY {
		t.Fatalf("cny lowercase display got %v", got)
	}
}

func TestEffectiveUSDToCNYRateFromConfig(t *testing.T) {
	if got := config.EffectiveUSDToCNYRate(6.8); got != 6.8 {
		t.Fatalf("expected configured rate 6.8, got %v", got)
	}
	if got := config.EffectiveUSDToCNYRate(0); got != config.DefaultUSDToCNYRate {
		t.Fatalf("expected default rate, got %v", got)
	}
}
