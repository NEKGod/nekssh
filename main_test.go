package main

import (
	"errors"
	"testing"
)

func TestClassifySSHError(t *testing.T) {
	tests := []struct{ input, code string }{
		{"ssh: unable to authenticate, attempted methods [none password]", "AUTH_FAILED"},
		{"dial tcp: connect: connection refused", "CONNECTION_REFUSED"},
		{"dial tcp: i/o timeout", "TIMEOUT"},
		{"lookup example.invalid: no such host", "HOST_NOT_FOUND"},
		{"knownhosts: key mismatch", "HOST_KEY_FAILED"},
	}
	for _, test := range tests {
		if got := classifySSHError(errors.New(test.input)); got.Code != test.code {
			t.Errorf("classifySSHError(%q) code = %q, want %q", test.input, got.Code, test.code)
		}
	}
}

func TestValidateConnection(t *testing.T) {
	c := &connectRequest{Host: " server.local ", Username: " root ", Port: 22}
	if err := validate(c); err != nil {
		t.Fatal(err)
	}
	if c.Host != "server.local" || c.Username != "root" {
		t.Fatalf("values were not normalized: %#v", c)
	}
	if err := validate(&connectRequest{Host: "bad\nhost", Username: "root", Port: 22}); err == nil {
		t.Fatal("expected invalid host error")
	}
}
