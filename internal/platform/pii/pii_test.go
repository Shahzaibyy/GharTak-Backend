package pii_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/yourusername/ghartak-backend/internal/platform/pii"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr error
	}{
		{name: "local", raw: "03001234567", want: "+923001234567"},
		{name: "spaces", raw: "0300 123 4567", want: "+923001234567"},
		{name: "dashes", raw: "0300-123-4567", want: "+923001234567"},
		{name: "country", raw: "923001234567", want: "+923001234567"},
		{name: "plus", raw: "+923001234567", want: "+923001234567"},
		{name: "empty", raw: "", wantErr: pii.ErrInvalidPhone},
		{name: "landline", raw: "0512345678", wantErr: pii.ErrInvalidPhone},
		{name: "short", raw: "030012345", wantErr: pii.ErrInvalidPhone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pii.NormalizePhone(tt.raw)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %s", got)
			}
		})
	}
}

func TestSealPhone(t *testing.T) {
	enc := bytes32("enc-key-for-ghartak-pii-test!!")
	hash := bytes32("hash-key-for-ghartak-pii-test!")
	firstText, firstLookup, err := pii.SealPhone(enc, hash, "0300-1234567")
	if err != nil {
		t.Fatal(err)
	}
	secondText, secondLookup, err := pii.SealPhone(enc, hash, "+923001234567")
	if err != nil {
		t.Fatal(err)
	}
	if firstLookup != secondLookup {
		t.Fatal("lookup changed across formats")
	}
	if len(firstLookup) != 64 || strings.ToLower(firstLookup) != firstLookup {
		t.Fatalf("lookup = %s", firstLookup)
	}
	if firstText == secondText {
		t.Fatal("ciphertext was not randomized")
	}
	opened, err := pii.Decrypt(enc, firstText)
	if err != nil {
		t.Fatal(err)
	}
	if opened != "+923001234567" {
		t.Fatalf("opened = %s", opened)
	}
	if _, err := pii.Decrypt(enc, firstText[:len(firstText)-2]+"aa"); !errors.Is(err, pii.ErrInvalidCiphertext) {
		t.Fatalf("tamper err = %v", err)
	}
}

func TestLookupChangesWithKey(t *testing.T) {
	phone := "+923001234567"
	left, err := pii.PhoneLookup(bytes32("hash-key-for-ghartak-pii-test!"), phone)
	if err != nil {
		t.Fatal(err)
	}
	right, err := pii.PhoneLookup(bytes32("other-key-for-ghartak-pii-test"), phone)
	if err != nil {
		t.Fatal(err)
	}
	if left == right {
		t.Fatal("lookup ignored the key")
	}
}

func bytes32(raw string) []byte {
	buf := make([]byte, 32)
	copy(buf, raw)
	return buf
}

func TestKeyLength(t *testing.T) {
	short, err := base64.StdEncoding.DecodeString("aaaa")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pii.PhoneLookup(short, "+923001234567"); !errors.Is(err, pii.ErrInvalidKey) {
		t.Fatalf("err = %v", err)
	}
}
