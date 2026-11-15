package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeMCP meniru Penpot MCP: SSE responses, Mcp-Session-Id, 400 untuk sesi
// tak dikenal, dan bisa "melupakan" sesi untuk menguji re-initialize.
type fakeMCP struct {
	mu       sync.Mutex
	sessions map[string]bool
	n        int
	inits    int
	calls    int
	uaSeen   string
}

func (f *fakeMCP) forget() { f.mu.Lock(); f.sessions = map[string]bool{}; f.mu.Unlock() }

func (f *fakeMCP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uaSeen = r.Header.Get("User-Agent")
	if r.Method == "DELETE" {
		delete(f.sessions, r.Header.Get("Mcp-Session-Id"))
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req struct {
		ID     *int64          `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	json.Unmarshal(body, &req)
	sid := r.Header.Get("Mcp-Session-Id")
	if req.Method == "initialize" {
		f.n++
		f.inits++
		sid = fmt.Sprintf("sess-%d", f.n)
		f.sessions[sid] = true
		w.Header().Set("Mcp-Session-Id", sid)
	} else if !f.sessions[sid] {
		http.Error(w, `{"error":"Bad Request: No valid session ID provided"}`, 400)
		return
	}
	if req.ID == nil { // notification
		w.WriteHeader(202)
		return
	}
	var result any
	switch req.Method {
	case "initialize":
		result = map[string]any{"serverInfo": map[string]any{"name": "penpot", "version": "1.0.0"}}
	case "tools/list":
		result = map[string]any{"tools": []any{map[string]any{"name": "execute_code"}, map[string]any{"name": "export_shape"}}}
	case "tools/call":
		f.calls++
		var p struct {
			Name string         `json:"name"`
			Args map[string]any `json:"arguments"`
		}
		json.Unmarshal(req.Params, &p)
		switch p.Name {
		case "execute_code":
			code, _ := p.Args["code"].(string)
			if strings.Contains(code, "boom") {
				result = map[string]any{"content": []any{map[string]any{"type": "text", "text": "Tool execution failed: Error: boom"}}}
			} else {
				result = map[string]any{"content": []any{map[string]any{"type": "text", "text": `{"result":{"ok":true},"log":""}`}}}
			}
		case "export_shape":
			big := strings.Repeat("A", 3<<20) // baris data SSE > 3 MB
			result = map[string]any{"content": []any{map[string]any{"type": "image", "mimeType": "image/png", "data": big}}}
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	resp, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": *req.ID, "result": result})
	fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\"}\n\nevent: message\ndata: %s\n\n", resp)
}

func TestSessionReuseAndReinit(t *testing.T) {
	f := &fakeMCP{sessions: map[string]bool{}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	s := newSession(srv.URL)

	for i := 0; i < 3; i++ {
		res, err := s.callTool("execute_code", map[string]any{"code": "return 1"}, 5*time.Second)
		if err != nil || res.IsError {
			t.Fatalf("call %d: %v %+v", i, err, res)
		}
	}
	if f.inits != 1 {
		t.Fatalf("sesi harus dipakai ulang: inits=%d", f.inits)
	}
	if !strings.HasPrefix(f.uaSeen, "penpot-cli/") {
		t.Fatalf("User-Agent tidak diset: %q", f.uaSeen)
	}

	f.forget() // server restart / sesi kedaluwarsa
	if _, err := s.callTool("execute_code", map[string]any{"code": "return 1"}, 5*time.Second); err != nil {
		t.Fatalf("harus re-initialize otomatis: %v", err)
	}
	if f.inits != 2 {
		t.Fatalf("re-init sekali: inits=%d", f.inits)
	}
}

func TestExecutionFailedIsError(t *testing.T) {
	f := &fakeMCP{sessions: map[string]bool{}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	res, err := newSession(srv.URL).callTool("execute_code", map[string]any{"code": "boom"}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatalf("'Tool execution failed:' harus jadi IsError: %+v", res)
	}
}

func TestLargeSSELine(t *testing.T) {
	f := &fakeMCP{sessions: map[string]bool{}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	res, err := newSession(srv.URL).callTool("export_shape", map[string]any{"shapeId": "page"}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Images) != 1 || len(res.Images[0].Data) != 3<<20 {
		t.Fatalf("gambar besar terpotong: %d", len(res.Images))
	}
}

func TestParseArgs(t *testing.T) {
	o, err := parseArgs([]string{"import", "--x", "10", "a.png", "--name=Ref", "--json", "--timeout", "5s"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(o.pos, ",") != "import,a.png" || o.flags["x"] != "10" || o.flags["name"] != "Ref" || !o.json || o.timeout != 5*time.Second {
		t.Fatalf("parse salah: %+v", o)
	}
	if o, _ := parseArgs([]string{"-f", "a.js"}, time.Second); o.flags["f"] != "a.js" || len(o.pos) != 0 {
		t.Fatalf("-f harus mengambil nilai: %+v", o)
	}
	if _, err := parseArgs([]string{"--x"}, time.Second); err == nil {
		t.Fatal("flag tanpa nilai harus error")
	}
}

func TestRedact(t *testing.T) {
	if got := redact("https://design.penpot.app/mcp/stream?userToken=SECRET"); strings.Contains(got, "SECRET") {
		t.Fatalf("token bocor: %s", got)
	}
}

func TestPinGuard(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if got := pinGuard("return 1"); got != "return 1" {
		t.Fatalf("tanpa pin harus apa adanya: %q", got)
	}
	os.MkdirAll(runtimeDir(), 0o700)
	os.WriteFile(pinPath(), []byte(`{"id":"abc","name":"Ads"}`), 0o600)
	g := pinGuard("return 1")
	if !strings.Contains(g, `penpot.currentFile?.id !== "abc"`) || !strings.HasSuffix(g, "return 1") {
		t.Fatalf("guard salah: %q", g)
	}
	t.Setenv("PENPOT_PIN", "off")
	if pinGuard("return 1") != "return 1" {
		t.Fatal("PENPOT_PIN=off harus mematikan guard")
	}
}
