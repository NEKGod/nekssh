package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReflectAgentWritesFileAndRunsCommand(t *testing.T) {
	reflectWorkspaceDir = t.TempDir()
	app := NewApp()
	server := httptest.NewServer(app.reflectMux(""))
	t.Cleanup(server.Close)

	index, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer index.Body.Close()
	indexBody, _ := io.ReadAll(index.Body)
	if index.StatusCode != http.StatusOK || !strings.Contains(string(indexBody), "/v1/run") {
		t.Fatalf("index %d %s", index.StatusCode, indexBody)
	}

	write, err := http.Post(server.URL+"/v1/files", "application/json", strings.NewReader(`{"path":"hello.txt","content":"nekssh"}`))
	if err != nil {
		t.Fatal(err)
	}
	write.Body.Close()
	if write.StatusCode != http.StatusOK {
		t.Fatalf("write status %d", write.StatusCode)
	}
	data, err := os.ReadFile(filepath.Join(reflectWorkspaceDir, "hello.txt"))
	if err != nil || string(data) != "nekssh" {
		t.Fatalf("file %q err %v", data, err)
	}

	blocked := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/files", strings.NewReader(`{"path":"../outside.txt","content":"no"}`))
	app.reflectWriteFiles(blocked, req)
	if blocked.Code != http.StatusBadRequest {
		t.Fatalf("escape status %d", blocked.Code)
	}

	runBody := bytes.NewBufferString(`{"command":"echo nekssh-job"}`)
	run, err := http.Post(server.URL+"/v1/run", "application/json", runBody)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Body.Close()
	runText, _ := io.ReadAll(run.Body)
	if run.StatusCode != http.StatusOK || !strings.Contains(string(runText), `"id":"job-1"`) {
		t.Fatalf("run %d %s", run.StatusCode, runText)
	}
}
