package pii_test

import (
	"errors"
	"testing"

	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

func TestNormalizeCNIC(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr error
	}{
		{name: "dashes", raw: "12345-1234567-1", want: "1234512345671"},
		{name: "digits", raw: "1234512345671", want: "1234512345671"},
		{name: "spaces", raw: "12345 1234567 1", want: "1234512345671"},
		{name: "short", raw: "12345-1234567", wantErr: pii.ErrInvalidCNIC},
		{name: "letters", raw: "12345-1234567-A", wantErr: pii.ErrInvalidCNIC},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pii.NormalizeCNIC(tt.raw)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %s err %v", got, err)
			}
		})
	}
}
