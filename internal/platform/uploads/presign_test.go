package uploads

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

func TestPresignPurposes(t *testing.T) {
	owner := uuid.MustParse("44444444-4444-4444-8444-444444444401")
	signer := NewSigner(Settings{Endpoint: "http://127.0.0.1:9000", Bucket: "ghartak"})
	tests := []struct {
		purpose  string
		prefix   string
		maxBytes int64
		wantErr  error
	}{
		{purpose: "cnic", prefix: "cnic/", maxBytes: 2 << 20},
		{purpose: "selfie", prefix: "selfie/", maxBytes: 2 << 20},
		{purpose: "vehicle_doc", prefix: "vehicle/", maxBytes: 5 << 20},
		{purpose: "catalog_photo", prefix: "catalog/", maxBytes: 8 << 20},
		{purpose: "proof_of_delivery", prefix: "proof/", maxBytes: 8 << 20},
		{purpose: "other", wantErr: apperror.ErrInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.purpose, func(t *testing.T) {
			result, err := signer.Presign(context.Background(), tt.purpose, owner)
			if tt.wantErr != nil {
				if err == nil || !strings.Contains(err.Error(), "purpose") {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(result.ObjectKey, tt.prefix+owner.String()+"/") || result.MaxBytes != tt.maxBytes {
				t.Fatalf("result = %+v", result)
			}
			if result.ExpiresIn != expiresSec || result.ContentType != "image/jpeg" || !strings.Contains(result.UploadURL, result.ObjectKey) {
				t.Fatalf("url = %s", result.UploadURL)
			}
		})
	}
}

func TestSignedURLIsStable(t *testing.T) {
	when := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	signer := NewSigner(Settings{
		Endpoint: "http://127.0.0.1:9000", Bucket: "ghartak", Region: "us-east-1",
		AccessKey: "access", SecretKey: "secret", Now: func() time.Time { return when },
	})
	url, err := signer.signedURL("cnic/owner/file", "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url, "X-Amz-Algorithm=AWS4-HMAC-SHA256") || !strings.Contains(url, "X-Amz-Signature=") {
		t.Fatalf("url = %s", url)
	}
	again, err := signer.signedURL("cnic/owner/file", "image/jpeg")
	if err != nil || again != url {
		t.Fatalf("again = %s err = %v", again, err)
	}
}
