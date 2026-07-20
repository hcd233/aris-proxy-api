package tracecli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type HTTPClient struct {
	client *http.Client
}

func NewHTTPClient(client *http.Client) *HTTPClient {
	if client == nil {
		client = &http.Client{Timeout: constant.TraceClientHTTPTimeout}
	}
	return &HTTPClient{client: client}
}

func normalizeHost(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", ierr.Wrap(ierr.ErrValidation, err, "parse trace server host")
	}
	isHTTPScheme := parsed.Scheme == constant.TraceClientSchemeHTTP ||
		parsed.Scheme == constant.TraceClientSchemeHTTPS
	hasOriginOnly := parsed.Host != "" &&
		(parsed.Path == "" || parsed.Path == constant.RoutePathRoot) &&
		parsed.RawQuery == "" && parsed.Fragment == "" && parsed.User == nil
	if !isHTTPScheme || !hasOriginOnly {
		return "", ierr.New(ierr.ErrValidation, "trace server host must be an http or https origin")
	}
	return strings.TrimSuffix(parsed.String(), constant.RoutePathRoot), nil
}

func (c *HTTPClient) CheckHealth(ctx context.Context, host string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		host+constant.RoutePathHealth,
		http.NoBody,
	)
	if err != nil {
		return ierr.Wrap(ierr.ErrBadRequest, err, "create trace health request")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return ierr.Wrap(ierr.ErrProxySend, err, "send trace health request")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close
	if resp.StatusCode != http.StatusOK {
		return ierr.New(
			ierr.ErrBadRequest,
			fmt.Sprintf("trace health request failed with status %d", resp.StatusCode),
		)
	}
	return nil
}

func (c *HTTPClient) CheckAPIKey(ctx context.Context, host, apiKey string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		host+constant.TraceClientCheckPath,
		http.NoBody,
	)
	if err != nil {
		return ierr.Wrap(ierr.ErrBadRequest, err, "create trace api key request")
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return ierr.Wrap(ierr.ErrProxySend, err, "send trace api key request")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close
	if resp.StatusCode != http.StatusNoContent {
		return ierr.New(
			ierr.ErrBadRequest,
			fmt.Sprintf("trace api key request failed with status %d", resp.StatusCode),
		)
	}
	return nil
}
