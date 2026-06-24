package util

import (
	"testing"
)

func TestDetectChromeBrand_Chrome124(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	brand := detectChromeBrand(ua)
	if brand == "" {
		t.Fatal("expected a brand string")
	}
	if brand != `"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"` {
		t.Fatalf("unexpected brand for Chrome 124: %s", brand)
	}
}

func TestDetectChromeBrand_Safari(t *testing.T) {
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15"
	brand := detectChromeBrand(ua)
	if brand == "" {
		t.Fatal("expected a brand string for Safari")
	}
}

func TestDetectChromeBrand_Empty(t *testing.T) {
	brand := detectChromeBrand("")
	if brand != "" {
		t.Fatalf("expected empty for empty UA, got: %s", brand)
	}
}

func TestDetectPlatform_Mac(t *testing.T) {
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"
	plat := detectPlatform(ua)
	if plat != `"macOS"` {
		t.Fatalf("expected macOS, got: %s", plat)
	}
}

func TestDetectPlatform_Linux(t *testing.T) {
	ua := "Mozilla/5.0 (X11; Linux x86_64)"
	plat := detectPlatform(ua)
	if plat != `"Linux"` {
		t.Fatalf("expected Linux, got: %s", plat)
	}
}

func TestDetectPlatform_Windows(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"
	plat := detectPlatform(ua)
	if plat != `"Windows"` {
		t.Fatalf("expected Windows, got: %s", plat)
	}
}
