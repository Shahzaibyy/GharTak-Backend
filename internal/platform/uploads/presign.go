package uploads

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
)

const expiresSec = 900

type purposeRule struct {
	prefix      string
	contentType string
	maxBytes    int64
}

var purposeRules = map[string]purposeRule{
	"cnic":              {prefix: "cnic", contentType: "image/jpeg", maxBytes: 2 << 20},
	"selfie":            {prefix: "selfie", contentType: "image/jpeg", maxBytes: 2 << 20},
	"vehicle_doc":       {prefix: "vehicle", contentType: "image/jpeg", maxBytes: 5 << 20},
	"catalog_photo":     {prefix: "catalog", contentType: "image/jpeg", maxBytes: 8 << 20},
	"proof_of_delivery": {prefix: "proof", contentType: "image/jpeg", maxBytes: 8 << 20},
}

type Settings struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	Now       func() time.Time
}

type Result struct {
	UploadURL   string `json:"upload_url"`
	ObjectKey   string `json:"object_key"`
	ExpiresIn   int    `json:"expires_in"`
	ContentType string `json:"content_type"`
	MaxBytes    int64  `json:"max_bytes"`
}

type Signer struct {
	Settings
}

func NewSigner(settings Settings) Signer {
	if settings.Now == nil {
		settings.Now = time.Now
	}
	return Signer{Settings: settings}
}

func (s Signer) Presign(_ context.Context, purpose string, ownerID uuid.UUID) (Result, error) {
	rule, err := ruleFor(purpose)
	if err != nil {
		return Result{}, err
	}
	key := objectKey(rule, ownerID)
	uploadURL, err := s.urlFor(key, rule.contentType)
	if err != nil {
		return Result{}, err
	}
	return Result{
		UploadURL: uploadURL, ObjectKey: key, ExpiresIn: expiresSec,
		ContentType: rule.contentType, MaxBytes: rule.maxBytes,
	}, nil
}

func ruleFor(purpose string) (purposeRule, error) {
	rule, ok := purposeRules[purpose]
	if !ok {
		return purposeRule{}, apperror.Invalid("purpose is invalid")
	}
	return rule, nil
}

func objectKey(rule purposeRule, ownerID uuid.UUID) string {
	return rule.prefix + "/" + ownerID.String() + "/" + uuid.NewString()
}

func (s Signer) urlFor(key, contentType string) (string, error) {
	if s.AccessKey == "" || s.SecretKey == "" {
		return s.devURL(key), nil
	}
	return s.signedURL(key, contentType)
}

func (s Signer) devURL(key string) string {
	return strings.TrimRight(s.Endpoint, "/") + "/" + s.Bucket + "/" + escapeKey(key)
}

func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
