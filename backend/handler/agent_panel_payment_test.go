package handler

import "testing"

func TestDiscountedAgentPrice(t *testing.T) {
	tests := []struct {
		name     string
		price    float64
		discount float64
		want     float64
	}{
		{name: "seven tenths", price: 10, discount: 7, want: 7},
		{name: "round to cents", price: 19.99, discount: 8.5, want: 16.99},
		{name: "free plan", price: 0, discount: 7, want: 0},
		{name: "invalid discount uses full price", price: 10, discount: 0, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := discountedAgentPrice(tt.price, tt.discount); got != tt.want {
				t.Fatalf("discountedAgentPrice(%v, %v) = %v, want %v", tt.price, tt.discount, got, tt.want)
			}
		})
	}
}
