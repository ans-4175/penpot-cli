package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Penpot MCP = remote "streamable HTTP" (bukan child process seperti Pen.app).
// "Persistent session" di sini berarti: initialize + tools/list sekali,
// Mcp-Session-Id disimpan dan dipakai ulang, koneksi HTTP keep-alive dipakai
// ulang, re-initialize otomatis kalau server lupa sesinya.

const (
	protocolVersion = "2025-03-26"
	maxBuf          = 64 << 20 // respons/request bisa besar (export PNG, upload referensi)
)

func userAgent() string { return "penpot-cli/" + version }

type toolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

type imagePart struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64
}

// toolResult = content[] MCP yang sudah dipilah.
type toolResult struct {
	Text    string      `json:"text,omitempty"`
	Images  []imagePart `json:"images,omitempty"`
	IsError bool        `json:"isError,omitempty"`
}

type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string {
	body := strings.TrimSpace(e.Body)
	if len(body) > 300 {
		body = body[:300] + "…"
	}
	switch e.Status {
	case 401, 403:
		return fmt.Sprintf("HTTP %d — token MCP ditolak. Regenerate di Penpot → Your account → Integrations → MCP Server, lalu update PENPOT_MCP_TOKEN di %s (%s)", e.Status, configPath(), body)
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, body)
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type mcpSession struct {
	endpoint string
	http     *http.Client

	mu         sync.Mutex // guards field di bawah
	sessionID  string
	nextID     int64
	tools      []toolInfo
	serverInfo string
	inits      int // jumlah initialize (1 = belum pernah re-init)
	calls      int

	callMu sync.Mutex // serialisasi: satu panggilan MCP dalam satu waktu
}

func newSession(endpoint string) *mcpSession {
	return &mcpSession{
		endpoint: endpoint,
		// Tanpa Timeout global — tiap request pakai context sendiri, supaya
		// timeout per-command (exec 120s, doctor 30s, ...) berlaku.
		http: &http.Client{Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     5 * time.Minute,
		}},
	}
}

// ---- config ----

func configPath() string { return filepath.Join(homeDir(), ".config", "ans", "penpot") }

// resolveEndpoint: PENPOT_MCP_URL (full, mis. MCP lokal http://localhost:4401/mcp)
// > PENPOT_DOMAIN + PENPOT_MCP_TOKEN dari env > dari ~/.config/ans/penpot.
func resolveEndpoint() (string, error) {
	if u := os.Getenv("PENPOT_MCP_URL"); u != "" {
		return u, nil
	}
	cfg := map[string]string{}
	if data, err := os.ReadFile(configPath()); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if i := strings.Index(line, "="); i > 0 {
				cfg[strings.TrimSpace(line[:i])] = strings.Trim(strings.TrimSpace(line[i+1:]), `"'`)
			}
		}
	}
	get := func(k string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return cfg[k]
	}
	if u := get("PENPOT_MCP_URL"); u != "" {
		return u, nil
	}
	domain, token := get("PENPOT_DOMAIN"), get("PENPOT_MCP_TOKEN")
	if domain == "" {
		domain = "design.penpot.app"
	}
	if token == "" {
		return "", fmt.Errorf("PENPOT_MCP_TOKEN tidak ada — isi di %s (Penpot → Your account → Integrations → MCP Server)", configPath())
	}
	return fmt.Sprintf("https://%s/mcp/stream?userToken=%s", domain, url.QueryEscape(token)), nil
}

// redact membuang query string (token) — endpoint aman untuk log/output.
func redact(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "(endpoint)"
	}
	u.RawQuery = ""
	return u.String()
}

// ---- transport ----

func (s *mcpSession) post(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", s.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", userAgent()) // tanpa UA wajar server menjawab 403
	s.mu.Lock()
	if s.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", s.sessionID)
		req.Header.Set("Mcp-Protocol-Version", protocolVersion)
	}
	s.mu.Unlock()
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		s.mu.Lock()
		s.sessionID = sid
		s.mu.Unlock()
	}
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		return nil, &httpError{Status: resp.StatusCode, Body: string(data)}
	}
	return resp, nil
}

func (s *mcpSession) rpc(ctx context.Context, method string, params any) (json.RawMessage, error) {
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	s.mu.Unlock()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	resp, err := s.post(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var r *rpcResponse
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		r, err = readSSE(resp.Body, id)
	} else {
		r = &rpcResponse{}
		err = json.NewDecoder(io.LimitReader(resp.Body, maxBuf)).Decode(r)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	if r.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", r.Error.Code, r.Error.Message)
	}
	return r.Result, nil
}

func (s *mcpSession) notify(ctx context.Context, method string) error {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method})
	resp, err := s.post(ctx, body)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

// readSSE membaca event stream sampai ketemu respons dengan id yang dicari.
// Pakai ReadString (bukan Scanner) supaya baris data raksasa (PNG base64)
// tidak kena batas token.
func readSSE(r io.Reader, id int64) (*rpcResponse, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	want := fmt.Sprint(id)
	var data strings.Builder
	dispatch := func() *rpcResponse {
		defer data.Reset()
		if data.Len() == 0 {
			return nil
		}
		var m rpcResponse
		if json.Unmarshal([]byte(data.String()), &m) != nil {
			return nil
		}
		if strings.Trim(string(m.ID), `"`) == want {
			return &m
		}
		return nil // notification / respons lain — abaikan
	}
	for {
		line, err := br.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if m := dispatch(); m != nil {
				return m, nil
			}
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
		if err != nil {
			if m := dispatch(); m != nil {
				return m, nil
			}
			if errors.Is(err, io.EOF) {
				return nil, errors.New("stream MCP tertutup tanpa respons yang cocok")
			}
			return nil, err
		}
	}
}

// ---- session lifecycle ----

// connect = handshake penuh: initialize → notifications/initialized →
// tools/list (di-cache). Dipanggil saat daemon start dan saat re-init.
func (s *mcpSession) connect(ctx context.Context) error {
	s.mu.Lock()
	s.sessionID = ""
	s.mu.Unlock()
	res, err := s.rpc(ctx, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "penpot-cli", "version": version},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	var init struct {
		ServerInfo struct{ Name, Version string } `json:"serverInfo"`
	}
	json.Unmarshal(res, &init)
	if err := s.notify(ctx, "notifications/initialized"); err != nil {
		return fmt.Errorf("notifications/initialized: %w", err)
	}
	lres, err := s.rpc(ctx, "tools/list", map[string]any{})
	if err != nil {
		return fmt.Errorf("tools/list: %w", err)
	}
	var list struct {
		Tools []toolInfo `json:"tools"`
	}
	if err := json.Unmarshal(lres, &list); err != nil {
		return fmt.Errorf("tools/list: %w", err)
	}
	s.mu.Lock()
	s.tools = list.Tools
	s.serverInfo = strings.TrimSpace(init.ServerInfo.Name + " " + init.ServerInfo.Version)
	s.inits++
	s.mu.Unlock()
	return nil
}

func (s *mcpSession) connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID != "" && s.tools != nil
}

// sessionLost: server lupa/menolak sesi → layak re-initialize sekali.
// (Penpot menjawab 400 untuk request tanpa sesi valid; spec MCP pakai 404.)
func sessionLost(err error) bool {
	var he *httpError
	return errors.As(err, &he) && (he.Status == 404 || he.Status == 400)
}

// callTool menjalankan satu tool. Serial (callMu). Timeout hanya
// membatalkan request HTTP ini — sesi tetap hidup. Catatan: kode yang
// sudah dikirim ke plugin bisa tetap jalan walau request dibatalkan.
func (s *mcpSession) callTool(name string, args map[string]any, timeout time.Duration) (*toolResult, error) {
	s.callMu.Lock()
	defer s.callMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if !s.connected() {
		if err := s.connect(ctx); err != nil {
			return nil, wrapCtx(ctx, err, timeout)
		}
	}
	if args == nil {
		args = map[string]any{}
	}
	params := map[string]any{"name": name, "arguments": args}
	res, err := s.rpc(ctx, "tools/call", params)
	if err != nil && sessionLost(err) {
		if cerr := s.connect(ctx); cerr != nil {
			return nil, wrapCtx(ctx, fmt.Errorf("re-initialize gagal: %w", cerr), timeout)
		}
		res, err = s.rpc(ctx, "tools/call", params)
	}
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if err != nil {
		return nil, wrapCtx(ctx, err, timeout)
	}
	return parseToolResult(res)
}

func wrapCtx(ctx context.Context, err error, timeout time.Duration) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("timeout %s — sesi tetap hidup, tapi kode bisa saja sudah jalan di plugin: cek state dulu sebelum mengulang (%v)", timeout, err)
	}
	return err
}

func parseToolResult(res json.RawMessage) (*toolResult, error) {
	var out struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Data     string `json:"data"`
			MimeType string `json:"mimeType"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, fmt.Errorf("respons tool tidak bisa dibaca: %w", err)
	}
	tr := &toolResult{IsError: out.IsError}
	var texts []string
	for _, c := range out.Content {
		switch c.Type {
		case "text":
			texts = append(texts, c.Text)
		case "image":
			tr.Images = append(tr.Images, imagePart{MimeType: c.MimeType, Data: c.Data})
		}
	}
	tr.Text = strings.Join(texts, "\n")
	// Quirk Penpot MCP: kegagalan eksekusi (exception JS, tab plugin
	// suspended, plugin tidak konek) datang sebagai teks biasa TANPA isError.
	if strings.HasPrefix(strings.TrimSpace(tr.Text), "Tool execution failed:") {
		tr.IsError = true
	}
	return tr, nil
}

// close mengakhiri sesi di server (DELETE, best effort).
func (s *mcpSession) close() {
	s.mu.Lock()
	sid := s.sessionID
	s.mu.Unlock()
	if sid == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "DELETE", s.endpoint, nil)
	if err != nil {
		return
	}
	req.Header.Set("Mcp-Session-Id", sid)
	req.Header.Set("User-Agent", userAgent())
	if resp, err := s.http.Do(req); err == nil {
		resp.Body.Close()
	}
}

func (s *mcpSession) info() (sid string, tools []toolInfo, server string, inits, calls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID, s.tools, s.serverInfo, s.inits, s.calls
}
