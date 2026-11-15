package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const idleTTL = 15 * time.Minute

func runtimeDir() string { return filepath.Join(homeDir(), ".penpot-cli") }
func sockPath() string   { return filepath.Join(runtimeDir(), "daemon.sock") }
func pidPath() string    { return filepath.Join(runtimeDir(), "daemon.pid") }
func logPath() string    { return filepath.Join(runtimeDir(), "daemon.log") }

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}
	return "."
}

// ---- protokol client <-> daemon (newline-delimited JSON) ----

// Tool khusus yang dijawab daemon sendiri:
//
//	"ping"      status daemon + sesi, tanpa menyentuh MCP
//	"tools"     daftar tools MCP (dari cache handshake; connect kalau belum)
//	"reconnect" paksa handshake ulang (sesi baru)
type cliReq struct {
	Tool       string         `json:"tool"`
	Args       map[string]any `json:"args,omitempty"`
	TimeoutSec int            `json:"timeout_sec"`
}

type cliResp struct {
	OK     bool        `json:"ok"`
	Result *toolResult `json:"result,omitempty"`
	Tools  []toolInfo  `json:"tools,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// ---- daemon ----

type daemon struct {
	sess      *mcpSession
	ln        net.Listener
	startedAt time.Time

	mu         sync.Mutex
	timer      *time.Timer
	stopping   bool
	lastClient time.Time // request client terakhir (untuk keepalive)
}

// runDaemon (subcommand internal "serve"): satu proses, satu sesi MCP,
// melayani banyak client lewat unix socket. Handshake dilakukan malas
// (lazy) pada request pertama, supaya error token/jaringan sampai ke client
// sebagai pesan jelas, bukan "daemon gagal start".
func runDaemon() error {
	if err := os.MkdirAll(runtimeDir(), 0o700); err != nil {
		return err
	}
	if c, err := net.Dial("unix", sockPath()); err == nil {
		c.Close()
		return errors.New("daemon sudah jalan (socket hidup)")
	}
	os.Remove(sockPath()) // socket basi

	endpoint, err := resolveEndpoint()
	if err != nil {
		return err
	}

	ln, err := net.Listen("unix", sockPath())
	if err != nil {
		return fmt.Errorf("gagal listen %s: %w", sockPath(), err)
	}
	if logf, err := os.OpenFile(logPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		log.SetOutput(logf)
	}
	log.Printf("daemon start pid=%d endpoint=%s", os.Getpid(), redact(endpoint))

	d := &daemon{sess: newSession(endpoint), ln: ln, startedAt: time.Now()}
	os.WriteFile(pidPath(), []byte(fmt.Sprint(os.Getpid())), 0o600)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() { d.shutdown(fmt.Sprintf("terima sinyal %s", <-sig)) }()

	d.mu.Lock()
	d.timer = time.AfterFunc(idleTTL, func() { d.shutdown("idle " + idleTTL.String()) })
	d.lastClient = time.Now()
	d.mu.Unlock()
	d.startKeepalive()

	for {
		conn, err := ln.Accept()
		if err != nil {
			d.mu.Lock()
			stopping := d.stopping
			d.mu.Unlock()
			if !stopping {
				d.shutdown("accept error: " + err.Error())
			}
			break
		}
		go d.handleConn(conn)
	}

	d.sess.close()
	os.Remove(sockPath())
	os.Remove(pidPath())
	log.Printf("daemon stop")
	return nil
}

func (d *daemon) touch() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastClient = time.Now()
	if d.timer != nil {
		d.timer.Reset(idleTTL)
	}
}

func (d *daemon) shutdown(reason string) {
	d.mu.Lock()
	if d.stopping {
		d.mu.Unlock()
		return
	}
	d.stopping = true
	d.mu.Unlock()
	log.Printf("daemon berhenti: %s", reason)
	d.ln.Close()
}

func (d *daemon) handleConn(conn net.Conn) {
	defer conn.Close()
	br := bufio.NewReaderSize(conn, 64<<10)
	for {
		line, err := br.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" {
			var req cliReq
			if jerr := json.Unmarshal([]byte(line), &req); jerr != nil {
				writeResp(conn, cliResp{Error: "request tidak valid: " + jerr.Error()})
				return
			}
			d.touch()
			writeResp(conn, d.handle(req))
		}
		if err != nil {
			return
		}
	}
}

func (d *daemon) handle(req cliReq) cliResp {
	timeout := time.Duration(req.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	switch req.Tool {
	case "ping":
		sid, tools, server, inits, calls := d.sess.info()
		ka := "off"
		if os.Getenv("PENPOT_KEEPALIVE") != "0" {
			ka = keepaliveEvery.String()
		}
		state := "belum handshake"
		if sid != "" {
			state = fmt.Sprintf("session=%s… server=%q tools=%d inits=%d", short(sid), server, len(tools), inits)
		}
		return cliResp{OK: true, Result: &toolResult{Text: fmt.Sprintf(
			"pong pid=%d uptime=%s calls=%d keepalive=%s endpoint=%s %s", os.Getpid(),
			time.Since(d.startedAt).Round(time.Second), calls, ka, redact(d.sess.endpoint), state)}}

	case "tools", "reconnect":
		d.sess.callMu.Lock()
		var err error
		if req.Tool == "reconnect" || !d.sess.connected() {
			ctx, cancel := ctxTimeout(timeout)
			err = d.sess.connect(ctx)
			cancel()
		}
		d.sess.callMu.Unlock()
		if err != nil {
			log.Printf("%s gagal: %v", req.Tool, err)
			return cliResp{Error: err.Error()}
		}
		_, tools, _, _, _ := d.sess.info()
		return cliResp{OK: true, Tools: tools}
	}

	t0 := time.Now()
	res, err := d.sess.callTool(req.Tool, req.Args, timeout)
	if err != nil {
		log.Printf("call %s gagal (%s): %v", req.Tool, time.Since(t0).Round(time.Millisecond), err)
		return cliResp{Error: err.Error()}
	}
	return cliResp{OK: true, Result: res}
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func writeResp(conn net.Conn, r cliResp) {
	data, _ := json.Marshal(r)
	conn.Write(append(data, '\n'))
}

func readPid() (int, error) {
	b, err := os.ReadFile(pidPath())
	if err != nil {
		return 0, err
	}
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &pid); err != nil {
		return 0, err
	}
	return pid, nil
}
