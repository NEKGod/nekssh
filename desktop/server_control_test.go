package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerControlWithoutSession(t *testing.T) {
	app := NewApp()
	server := httptest.NewServer(app.serverControlMux())
	t.Cleanup(server.Close)

	index, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer index.Body.Close()
	body, _ := io.ReadAll(index.Body)
	if index.StatusCode != http.StatusOK || !strings.Contains(string(body), `"service":"nekssh-server"`) {
		t.Fatalf("index %d %s", index.StatusCode, body)
	}

	docs, err := http.Get(server.URL + "/docs")
	if err != nil {
		t.Fatal(err)
	}
	defer docs.Body.Close()
	docsBody, _ := io.ReadAll(docs.Body)
	if !strings.Contains(string(docsBody), "18766") || !strings.Contains(string(docsBody), "/v1/forward") {
		t.Fatalf("docs %s", docsBody)
	}

	response, err := http.Post(server.URL+"/v1/run", "application/json", strings.NewReader(`{"command":"uname -a"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		text, _ := io.ReadAll(response.Body)
		t.Fatalf("run %d %s", response.StatusCode, text)
	}
}

func TestRemoteControlPath(t *testing.T) {
	got, err := remoteControlPath("/tmp/a.txt")
	if err != nil || got != "/tmp/a.txt" {
		t.Fatalf("path %s %v", got, err)
	}
	if _, err = remoteControlPath("../etc/passwd"); err == nil {
		t.Fatal("accepted escaped path")
	}
}
