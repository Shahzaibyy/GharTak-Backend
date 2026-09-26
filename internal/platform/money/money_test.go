package money_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/yourusername/ghartak-backend/internal/platform/money"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		paisa   int64
		text    string
		wantErr error
	}{
		{name: "rupees", raw: "60", paisa: 6000, text: "60.00"},
		{name: "paisa", raw: "60.50", paisa: 6050, text: "60.50"},
		{name: "one decimal", raw: "60.5", paisa: 6050, text: "60.50"},
		{name: "zero", raw: "0", paisa: 0, text: "0.00"},
		{name: "leading zero", raw: "0.01", paisa: 1, text: "0.01"},
		{name: "negative", raw: "-1.25", paisa: -125, text: "-1.25"},
		{name: "empty", raw: "", wantErr: money.ErrInvalid},
		{name: "letters", raw: "abc", wantErr: money.ErrInvalid},
		{name: "three decimals", raw: "1.234", wantErr: money.ErrInvalid},
		{name: "trailing dot", raw: "60.", wantErr: money.ErrInvalid},
		{name: "space", raw: "60.00 ", wantErr: money.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := money.Parse(tt.raw)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got.Paisa() != tt.paisa {
				t.Fatalf("paisa = %d, want %d", got.Paisa(), tt.paisa)
			}
			if got.String() != tt.text {
				t.Fatalf("text = %s, want %s", got.String(), tt.text)
			}
		})
	}
}

func TestJSONRoundTrip(t *testing.T) {
	original, err := money.Parse("18.00")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `"18.00"` {
		t.Fatalf("json = %s", body)
	}
	var decoded money.Money
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(original) {
		t.Fatalf("decoded = %s", decoded)
	}
}

func TestAddSub(t *testing.T) {
	left, err := money.Parse("60.00")
	if err != nil {
		t.Fatal(err)
	}
	right, err := money.Parse("18.50")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := left.Add(right)
	if err != nil {
		t.Fatal(err)
	}
	if sum.String() != "78.50" {
		t.Fatalf("sum = %s", sum)
	}
	diff, err := sum.Sub(right)
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Equal(left) {
		t.Fatalf("diff = %s", diff)
	}
}

func TestNegMinInt(t *testing.T) {
	value := money.FromPaisa(math.MinInt64)
	if _, err := value.Neg(); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("err = %v", err)
	}
}

func TestMulDivHalfUp(t *testing.T) {
	one := money.FromPaisa(1)
	tests := []struct {
		name string
		mul  int64
		div  int64
		want int64
	}{
		{name: "half up", mul: 5000, div: 10000, want: 1},
		{name: "below half", mul: 4999, div: 10000, want: 0},
		{name: "exact", mul: 2, div: 1, want: 2},
		{name: "quantity", mul: 3, div: 1, want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := one.MulDiv(tt.mul, tt.div)
			if err != nil {
				t.Fatal(err)
			}
			if got.Paisa() != tt.want {
				t.Fatalf("paisa = %d", got.Paisa())
			}
		})
	}
}

func TestCmp(t *testing.T) {
	low, _ := money.Parse("1.00")
	high, _ := money.Parse("2.00")
	if low.Cmp(high) != -1 || high.Cmp(low) != 1 || low.Cmp(low) != 0 {
		t.Fatal("cmp")
	}
}
