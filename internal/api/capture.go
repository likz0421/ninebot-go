package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Traffic capture (protocol calibration aid).
//
// Set NINEBOT_CAPTURE_DIR (or call SetCaptureDir) to make every API
// exchange append one JSON line to <dir>/capture.jsonl. Encrypted-channel
// records include the per-request key material (KReq + the four keyData
// strings) so that failed response decryptions can be replayed offline —
// this is the artifact the M2 DeriveKey calibration needs.
//
// Capture is silent and off by default; enabling it does not alter any
// request or the CLI surface.

var (
	captureMu  sync.Mutex
	captureDir = os.Getenv("NINEBOT_CAPTURE_DIR")
)

// SetCaptureDir overrides the capture directory (empty disables capture).
func SetCaptureDir(dir string) {
	captureMu.Lock()
	defer captureMu.Unlock()
	captureDir = dir
}

func captureEnabled() bool {
	captureMu.Lock()
	defer captureMu.Unlock()
	return captureDir != ""
}

type captureRecord struct {
	TS             string            `json:"ts"`
	Channel        string            `json:"channel"`
	URL            string            `json:"url"`
	RequestHeaders map[string]string `json:"request_headers,omitempty"`
	RequestBody    string            `json:"request_body,omitempty"`
	KeyData        *captureKeyData   `json:"key_data,omitempty"`
	InnerPlain     string            `json:"inner_plain,omitempty"`
	ResponseStatus int               `json:"response_status"`
	ResponseBody   string            `json:"response_body,omitempty"`
	ResponsePlain  string            `json:"response_plain,omitempty"`
	Note           string            `json:"note,omitempty"`
}

type captureKeyData struct {
	KReq  string `json:"kreq"`
	One   string `json:"one"`
	Two   string `json:"two"`
	Three string `json:"three"`
	Four  string `json:"four"`
}

func capture(rec captureRecord) {
	captureMu.Lock()
	dir := captureDir
	captureMu.Unlock()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	rec.TS = time.Now().Format(time.RFC3339Nano)
	line, err := json.Marshal(rec)
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "capture.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

// flattenHeader snapshots an http.Header into a plain map (first value).
func flattenHeader(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
