package main

import "testing"

func TestAppVersionShowsProductAndBuild(t *testing.T) {
	version := AppVersion()
	if version == "" || version[:len(productVersion)] != productVersion {
		t.Fatalf("version = %q", version)
	}
	title := appTitle()
	if title != "NekSSH "+version {
		t.Fatalf("title = %q", title)
	}
}
