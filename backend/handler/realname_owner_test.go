package handler

import "testing"

func TestRealnamePendingOrderRoundTrip(t *testing.T) {
	want := realnamePendingOrder{
		OwnerType: "agent",
		OwnerID:   42,
		CertifyID: "certify-123",
		RealName:  "Test Name",
		IDCard:    "11010519491231002X",
	}

	got, err := parseRealnamePendingOrder(encodeRealnamePendingOrder(want))
	if err != nil {
		t.Fatalf("parseRealnamePendingOrder() error = %v", err)
	}
	if got != want {
		t.Fatalf("pending order = %#v, want %#v", got, want)
	}
}

func TestParseLegacyUserRealnamePendingOrder(t *testing.T) {
	got, err := parseRealnamePendingOrder("7|certify-old|Test Name|11010519491231002X")
	if err != nil {
		t.Fatalf("parseRealnamePendingOrder() error = %v", err)
	}
	if got.OwnerType != "user" || got.OwnerID != 7 || got.CertifyID != "certify-old" {
		t.Fatalf("legacy pending order parsed incorrectly: %#v", got)
	}
}
