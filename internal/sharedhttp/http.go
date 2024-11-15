package sharedhttp

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/avast/retry-go"
)

var Transport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   10,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	ReadBufferSize:        65536,
	WriteBufferSize:       65536,
	TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
	},
}

func CheckStatusCode(statusCode int) error {
	switch statusCode {
	case http.StatusOK:

	case http.StatusUnauthorized, http.StatusForbidden:
		return retry.Unrecoverable(fmt.Errorf("unrecoverable error downloading image: status code %d", statusCode))

	case http.StatusMethodNotAllowed:
		return retry.Unrecoverable(fmt.Errorf("method not allowed: status code %d", statusCode))

	case http.StatusNotFound:
		return fmt.Errorf("image not found - retrying: status code %d", statusCode)

	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusInternalServerError:
		return fmt.Errorf("server error encountered while downloading image: status code %d - retrying", statusCode)

	default:
		return retry.Unrecoverable(fmt.Errorf("unexpected error downloading image: status code %d", statusCode))
	}

	return nil
}

func ExecRequest(client http.Client, req *http.Request) (http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return http.Response{}, err
	}

	if err := CheckStatusCode(resp.StatusCode); err != nil {
		return http.Response{}, err
	}

	return *resp, nil
}
