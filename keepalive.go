package main

import (
	"log"
	"os"
	"strconv"
	"time"
)

// Keepalive: browser men-suspend tab plugin Penpot yang tidak terlihat
// setelah ±35–90 s tanpa aktivitas (Chrome Memory Saver / timer throttling).
// Ping ringan berkala (execute_code `return 1`) selama sesi kerja menjaga
// heartbeat plugin tetap hidup. Hanya berjalan selama daemon hidup dan
// berhenti sendiri setelah keepaliveFor tanpa request dari client, supaya
// tidak memaksa tab terus aktif ketika agent sudah tidak bekerja.
//
//	PENPOT_KEEPALIVE=0        matikan
//	PENPOT_KEEPALIVE=20s      interval (default 25s)
var (
	keepaliveEvery = 25 * time.Second
	keepaliveFor   = 10 * time.Minute
)

func (d *daemon) startKeepalive() {
	if v := os.Getenv("PENPOT_KEEPALIVE"); v != "" {
		if v == "0" || v == "off" {
			log.Printf("keepalive: dimatikan")
			return
		}
		if dur, err := time.ParseDuration(v); err == nil && dur >= 5*time.Second {
			keepaliveEvery = dur
		} else if n, err := strconv.Atoi(v); err == nil && n >= 5 {
			keepaliveEvery = time.Duration(n) * time.Second
		}
	}
	go func() {
		t := time.NewTicker(keepaliveEvery)
		defer t.Stop()
		for range t.C {
			d.mu.Lock()
			stopping := d.stopping
			last := d.lastClient
			d.mu.Unlock()
			if stopping {
				return
			}
			if !d.sess.connected() || time.Since(last) > keepaliveFor {
				continue // agent sedang tidak bekerja → biarkan tab tidur
			}
			if !d.sess.callMu.TryLock() {
				continue // ada panggilan berjalan — itu sudah heartbeat
			}
			d.sess.callMu.Unlock()
			if _, err := d.sess.callTool("execute_code", map[string]any{"code": "return 1"}, 20*time.Second); err != nil {
				log.Printf("keepalive: %v", err)
			}
		}
	}()
}
