package webfetch

import (
	"context"
	"io"
)

type HTTPClient interface {
	Do(context.Context, HTTPRequest) (HTTPResponse, error)
}

type Resolver interface {
	LookupIPAddr(context.Context, string) ([]string, error)
}

type URLPolicy interface {
	Validate(context.Context, string, bool) (string, error)
}

type HTTPRequest struct {
	URL    string
	Header map[string]string
}

type HTTPResponse struct {
	StatusCode int
	Header     map[string][]string
	Body       io.ReadCloser
}
