package money

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

var (
	ErrInvalid  = errors.New("invalid money")
	ErrOverflow = errors.New("money overflow")
)

// Money is an exact PKR amount in paisa. JSON is a decimal string such as "60.00".
type Money struct {
	paisa int64
}

func Parse(raw string) (Money, error) {
	negative, body, err := splitSign(raw)
	if err != nil {
		return Money{}, err
	}
	whole, frac, err := splitDecimal(body)
	if err != nil {
		return Money{}, err
	}
	return assemble(negative, whole, frac)
}

func FromPaisa(paisa int64) Money {
	return Money{paisa: paisa}
}

func (m Money) Paisa() int64 { return m.paisa }

func (m Money) IsZero() bool { return m.paisa == 0 }

func (m Money) IsNegative() bool { return m.paisa < 0 }

func (m Money) Equal(other Money) bool { return m.paisa == other.paisa }

func (m Money) Cmp(other Money) int {
	switch {
	case m.paisa < other.paisa:
		return -1
	case m.paisa > other.paisa:
		return 1
	default:
		return 0
	}
}

func (m Money) Add(other Money) (Money, error) {
	sum, ok := add(m.paisa, other.paisa)
	if !ok {
		return Money{}, ErrOverflow
	}
	return Money{paisa: sum}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	negated, err := other.Neg()
	if err != nil {
		return Money{}, err
	}
	return m.Add(negated)
}

// MulDiv multiplies by mul/div and rounds half up. div must be positive.
func (m Money) MulDiv(mul, div int64) (Money, error) {
	if mul < 0 || div <= 0 {
		return Money{}, ErrInvalid
	}
	return mulDiv(m.paisa, mul, div)
}

func (m Money) Neg() (Money, error) {
	if m.paisa == math.MinInt64 {
		return Money{}, ErrOverflow
	}
	return Money{paisa: -m.paisa}, nil
}

func (m Money) String() string {
	sign := ""
	value := m.paisa
	if value < 0 {
		sign = "-"
		value = -value
	}
	if value < 0 {
		return "-92233720368547758.08"
	}
	return fmt.Sprintf("%s%d.%02d", sign, value/100, value%100)
}

func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

func (m *Money) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return ErrInvalid
	}
	parsed, err := Parse(raw)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}

func splitSign(raw string) (bool, string, error) {
	if raw == "" || raw == "-" {
		return false, "", ErrInvalid
	}
	if raw[0] == '-' {
		return true, raw[1:], nil
	}
	return false, raw, nil
}

func splitDecimal(raw string) (string, string, error) {
	parts := strings.Split(raw, ".")
	if invalidParts(parts) {
		return "", "", ErrInvalid
	}
	if len(parts) == 1 {
		return parts[0], "00", nil
	}
	frac, err := padFrac(parts[1])
	if err != nil {
		return "", "", err
	}
	return parts[0], frac, nil
}

func invalidParts(parts []string) bool {
	if len(parts) > 2 {
		return true
	}
	return parts[0] == ""
}

func padFrac(frac string) (string, error) {
	if frac == "" || len(frac) > 2 {
		return "", ErrInvalid
	}
	if len(frac) == 1 {
		return frac + "0", nil
	}
	return frac, nil
}

func assemble(negative bool, whole string, frac string) (Money, error) {
	if !digitsOnly(whole) || !digitsOnly(frac) {
		return Money{}, ErrInvalid
	}
	return parseParts(negative, whole, frac)
}

func parseParts(negative bool, whole string, frac string) (Money, error) {
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return Money{}, ErrOverflow
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return Money{}, ErrInvalid
	}
	return scale(negative, w, f)
}

func scale(negative bool, whole int64, frac int64) (Money, error) {
	if whole > (math.MaxInt64-frac)/100 {
		return Money{}, ErrOverflow
	}
	paisa := whole*100 + frac
	if negative {
		paisa = -paisa
	}
	return Money{paisa: paisa}, nil
}

func digitsOnly(raw string) bool {
	if raw == "" {
		return false
	}
	for _, r := range raw {
		if !isDigit(r) {
			return false
		}
	}
	return true
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func mulDiv(paisa, mul, div int64) (Money, error) {
	num := big.NewInt(paisa)
	num.Mul(num, big.NewInt(mul))
	num.Add(num, big.NewInt(div/2))
	num.Div(num, big.NewInt(div))
	if !num.IsInt64() {
		return Money{}, ErrOverflow
	}
	return Money{paisa: num.Int64()}, nil
}

func add(a int64, b int64) (int64, bool) {
	sum := a + b
	if overflowed(a, b, sum) {
		return 0, false
	}
	return sum, true
}

func overflowed(a int64, b int64, sum int64) bool {
	if b > 0 && sum < a {
		return true
	}
	return b < 0 && sum > a
}
