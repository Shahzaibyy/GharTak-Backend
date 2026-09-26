package uploads

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

func (s Signer) signedURL(key, contentType string) (string, error) {
	endpoint, err := url.Parse(s.Endpoint)
	if err != nil || endpoint.Host == "" {
		return "", fmt.Errorf("uploads: endpoint is invalid")
	}
	now := s.Now().UTC()
	canonical := canonicalRequest(endpoint.Host, s.Bucket, key, contentType, s.query(now))
	signature := sign(s.SecretKey, s.Region, now, canonical)
	return endpoint.Scheme + "://" + endpoint.Host + "/" + s.Bucket + "/" + escapeKey(key) + "?" + s.query(now).Encode() + "&X-Amz-Signature=" + signature, nil
}

func (s Signer) query(now time.Time) url.Values {
	date := now.Format("20060102")
	values := url.Values{}
	values.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	values.Set("X-Amz-Credential", s.AccessKey+"/"+date+"/"+s.Region+"/s3/aws4_request")
	values.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	values.Set("X-Amz-Expires", fmt.Sprintf("%d", expiresSec))
	values.Set("X-Amz-SignedHeaders", "content-type;host")
	return values
}

func canonicalRequest(host, bucket, key, contentType string, query url.Values) string {
	lines := []string{
		"PUT",
		"/" + bucket + "/" + escapeKey(key),
		canonicalQuery(query),
		"content-type:" + contentType + "\nhost:" + host + "\n",
		"content-type;host",
		"UNSIGNED-PAYLOAD",
	}
	return strings.Join(lines, "\n")
}

func canonicalQuery(query url.Values) string {
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(query.Get(key)))
	}
	return strings.Join(parts, "&")
}

func sign(secret, region string, now time.Time, canonical string) string {
	date := now.Format("20060102")
	scope := date + "/" + region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + now.Format("20060102T150405Z") + "\n" + scope + "\n" + hexSHA(canonical)
	key := hmacSHA([]byte("AWS4"+secret), date)
	key = hmacSHA(key, region)
	key = hmacSHA(key, "s3")
	key = hmacSHA(key, "aws4_request")
	return hex.EncodeToString(hmacSHA(key, stringToSign))
}

func hexSHA(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func hmacSHA(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
