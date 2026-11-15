package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Batas body request MCP ±100 KB (HTTP 413). Kode yang lebih besar dari
// codeDirectMax dikirim bertahap ke storage.__code lalu dijalankan lewat
// eval dalam async function. `storage` tidak terlihat dari dalam eval
// (scope sandbox), jadi diteruskan sebagai parameter bersama penpot &
// penpotUtils.
const (
	codeDirectMax = 90 << 10 // byte JSON kode; di atas ini → chunked
	codeChunk     = 64 << 10
)

func codeSize(code string) int {
	b, _ := json.Marshal(code)
	return len(b)
}

// prepareCode: kode kecil dikembalikan apa adanya. Kode besar diunggah
// bertahap dan diganti dengan stub eval kecil.
func prepareCode(code string, timeout time.Duration) (string, error) {
	if codeSize(code) <= codeDirectMax {
		return code, nil
	}
	if _, _, err := runCodeDirect("storage.__code = []; return 0", timeout); err != nil {
		return "", err
	}
	parts := 0
	for i := 0; i < len(code); {
		end := min(i+codeChunk, len(code))
		for end < len(code) && end > i && !utf8Start(code[end]) { // jangan potong di tengah rune
			end--
		}
		q, _ := json.Marshal(code[i:end])
		if _, _, err := runCodeDirect("storage.__code.push("+string(q)+"); return storage.__code.length", timeout); err != nil {
			return "", fmt.Errorf("kirim potongan kode %d: %w", parts+1, err)
		}
		parts++
		i = end
	}
	fmt.Fprintf(os.Stderr, "[exec] kode %d KB > batas request, dikirim %d potongan\n", len(code)>>10, parts)
	return `const __src = storage.__code.join(''); delete storage.__code;
const __fn = eval('(async (penpot, penpotUtils, storage) => {\n' + __src + '\n})');
return await __fn(penpot, penpotUtils, storage);`, nil
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }
