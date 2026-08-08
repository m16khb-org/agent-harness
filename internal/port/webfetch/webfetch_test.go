package webfetch

import (
	"context"
	"io"
	"strings"
	"testing"
)

type resolverStub struct{}

func (resolverStub) LookupIPAddr(context.Context, string) ([]string, error) { return nil, nil }

type clientStub struct{}

func (clientStub) Do(context.Context, HTTPRequest) (HTTPResponse, error) {
	return HTTPResponse{Body: io.NopCloser(strings.NewReader(""))}, nil
}

func TestPortsAreCapabilityMinimal(t *testing.T) {
	var client HTTPClient = clientStub{}
	var resolver Resolver = resolverStub{}
	if client == nil || resolver == nil {
		t.Fatal("web-fetch ports must accept the standard client and a DNS resolver")
	}
}
