package services

import "testing"

func TestInferOrderType(t *testing.T) {
	tableID := "table-uuid-1"
	selfServiceID := "self-service-3"

	tests := []struct {
		name string
		req  CreateOrderRequest
		want string
	}{
		{
			name: "explicit counter",
			req:  CreateOrderRequest{OrderType: "counter"},
			want: "counter",
		},
		{
			name: "explicit dine in",
			req:  CreateOrderRequest{OrderType: "dine_in", TableID: &tableID},
			want: "dine_in",
		},
		{
			name: "legacy self service table id",
			req:  CreateOrderRequest{TableID: &selfServiceID},
			want: "counter",
		},
		{
			name: "takeaway customer name",
			req:  CreateOrderRequest{CustomerName: "Takeaway"},
			want: "counter",
		},
		{
			name: "dine in with table",
			req:  CreateOrderRequest{TableID: &tableID, TableNumber: "1A"},
			want: "dine_in",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := inferOrderType(tc.req); got != tc.want {
				t.Fatalf("inferOrderType() = %q, want %q", got, tc.want)
			}
		})
	}
}
