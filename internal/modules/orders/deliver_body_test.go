package orders

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeliverBodyAliases(t *testing.T) {
	otp, proof, err := deliverBody(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"otp":"4821","proof":"demo/key"}`)))
	if err != nil || otp != "4821" || proof != "demo/key" {
		t.Fatalf("otp=%q proof=%q err=%v", otp, proof, err)
	}
}

func TestDeliverBodyCanonical(t *testing.T) {
	otp, proof, err := deliverBody(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"delivery_otp":"1234","proof_photo_key":"pod/1"}`)))
	if err != nil || otp != "1234" || proof != "pod/1" {
		t.Fatalf("otp=%q proof=%q err=%v", otp, proof, err)
	}
}

func TestDeliverBodyEmpty(t *testing.T) {
	_, _, err := deliverBody(httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(nil)))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeliverBodyMissingFields(t *testing.T) {
	_, _, err := deliverBody(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"note":"hi"}`)))
	if err == nil {
		t.Fatal("expected error")
	}
}
