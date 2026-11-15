package main

import (
	"strings"
	"testing"
)

func TestChromeCandidates(t *testing.T) {
	env := map[string]string{"ProgramFiles": `C:\Program Files`, "LocalAppData": `C:\Users\u\AppData\Local`}
	got := chromeCandidates("windows", func(k string) string { return env[k] })
	if len(got) != 2 || !strings.HasSuffix(got[0], "chrome.exe") || !strings.Contains(got[0], "Program Files") {
		t.Fatalf("windows candidates: %v", got)
	}
	if lin := chromeCandidates("linux", func(string) string { return "" }); len(lin) == 0 || lin[0] != "google-chrome" {
		t.Fatalf("linux candidates: %v", lin)
	}
}

func TestWinRunningScriptEscapesQuotes(t *testing.T) {
	s := winRunningScript(`C:\Users\o'neil\.penpot-chrome`)
	if !strings.Contains(s, `o''neil`) || !strings.Contains(s, "--type=") {
		t.Fatalf("script: %s", s)
	}
}
