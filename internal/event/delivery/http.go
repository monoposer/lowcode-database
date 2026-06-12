package delivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTP delivers to https:// or http:// webhook endpoints.
type HTTP struct {
	Client *http.Client
}

func (h HTTP) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (h HTTP) Push(ctx context.Context, u *url.URL, msg Message) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(msg.Body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range msg.Headers {
		if strings.TrimSpace(k) != "" {
			req.Header.Set(k, v)
		}
	}
	if msg.Secret != "" {
		mac := hmac.New(sha256.New, []byte(msg.Secret))
		mac.Write(msg.Body)
		req.Header.Set("X-Lowcode-Signature", hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := h.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmtHTTPStatus(resp.Status)
	}
	return nil
}

type httpStatusError string

func (e httpStatusError) Error() string { return string(e) }

func fmtHTTPStatus(status string) error {
	return httpStatusError("webhook status " + status)
}

// Message is the payload handed to a delivery driver.
type Message struct {
	Body    []byte
	Headers map[string]string
	Secret  string
}

type missingParamError string

func (e missingParamError) Error() string {
	return fmt.Sprintf("delivery url missing %s", string(e))
}

type invalidParamError struct {
	key, val string
}

func (e invalidParamError) Error() string {
	return fmt.Sprintf("delivery url invalid %s=%q", e.key, e.val)
}

func errMissing(name string) error { return missingParamError(name) }
func errInvalid(key, val string) error {
	return invalidParamError{key: key, val: val}
}

func stripQuery(u *url.URL) string {
	clone := *u
	clone.RawQuery = ""
	clone.Fragment = ""
	return clone.String()
}
