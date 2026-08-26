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

// The ports must stay small enough that a plain HTTP client and a DNS resolver
// satisfy them; these assertions stop compiling if a method is added.
var (
	_ HTTPClient = clientStub{}
	_ Resolver   = resolverStub{}
)

func TestPortsAreCapabilityMinimal(t *testing.T) {
	var client HTTPClient = clientStub{}
	var resolver Resolver = resolverStub{}
	response, err := client.Do(context.Background(), HTTPRequest{})
	if err != nil || response.Body == nil {
		t.Fatalf("HTTPClient port must return a readable response: %+v %v", response, err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if addrs, err := resolver.LookupIPAddr(context.Background(), "example.invalid"); err != nil || addrs != nil {
		t.Fatalf("Resolver port must accept a hostname lookup: %v %v", addrs, err)
	}
}
