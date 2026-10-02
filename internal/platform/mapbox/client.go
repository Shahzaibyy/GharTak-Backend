package mapbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Settings struct {
	Token   string
	BaseURL string
	Timeout time.Duration
}

type Client struct {
	token   string
	base    string
	http    *http.Client
	breaker *Breaker
}

func NewClient(settings Settings) *Client {
	timeout := settings.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	base := strings.TrimRight(settings.BaseURL, "/")
	if base == "" {
		base = "https://api.mapbox.com"
	}
	return &Client{
		token:   settings.Token,
		base:    base,
		http:    &http.Client{Timeout: timeout},
		breaker: NewBreaker(5, 30*time.Second),
	}
}

func (c *Client) configured() bool {
	return c != nil && c.token != ""
}

func (c *Client) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	if !c.configured() {
		return nil, ErrUnavailable
	}
	if !c.breaker.Allow() {
		return nil, ErrUpstreamUnavailable
	}
	query.Set("access_token", c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.breaker.Fail()
		return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		c.breaker.Fail()
		return nil, fmt.Errorf("%w: read body", ErrUpstreamUnavailable)
	}
	return c.handleStatus(resp.StatusCode, body)
}

func (c *Client) handleStatus(status int, body []byte) ([]byte, error) {
	switch {
	case status == http.StatusOK:
		c.breaker.OK()
		return body, nil
	case status == http.StatusTooManyRequests:
		c.breaker.Fail()
		return nil, ErrRateLimited
	case status == http.StatusUnprocessableEntity || status == http.StatusBadRequest:
		return nil, ErrBadInput
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		c.breaker.Fail()
		return nil, ErrUnavailable
	default:
		c.breaker.Fail()
		return nil, ErrUpstreamUnavailable
	}
}

func encodeCoord(p Point) string {
	return formatFloat(p.Lng) + "," + formatFloat(p.Lat)
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func decodeJSON(body []byte, dst any) error {
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("%w: malformed body", ErrUpstreamUnavailable)
	}
	return nil
}
