package cmd

import (
	"crypto"
	"crypto/md5"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	nbcrypto "ninecli/internal/crypto"
)

// harVerifyCmd is a hidden protocol-calibration helper: it probes a HAR
// capture (real-device traffic) with the same primitives ninecli uses.
// It is intentionally hidden so the top-level help text stays identical
// to the reference binary's.
var harVerifyCmd = &cobra.Command{
	Use:    "har-verify <capture.har>",
	Short:  "Run protocol-calibration probes against a HAR capture",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		return runHarVerify(args[0])
	},
}

func init() {
	rootCmd.AddCommand(harVerifyCmd)
}

type harEntry struct {
	Request struct {
		Method  string `json:"method"`
		URL     string `json:"url"`
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
		PostData *struct {
			Text string `json:"text"`
		} `json:"postData"`
	} `json:"request"`
	Response struct {
		Status  int `json:"status"`
		Content struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"response"`
}

type harFile struct {
	Log struct {
		Entries []harEntry `json:"entries"`
	} `json:"log"`
}

func runHarVerify(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var h harFile
	if err := json.Unmarshal(raw, &h); err != nil {
		return fmt.Errorf("parse HAR: %w", err)
	}

	block, _ := pem.Decode([]byte(harVerifyPubPEM))
	if block == nil {
		return fmt.Errorf("bad embedded public key")
	}
	k, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}
	rsaPub, ok := k.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("embedded key is not RSA")
	}

	fmt.Println("=== [1] response s-field RSA verify (PKCS1v15 over passport pubkey) ===")
	verified, tried := 0, 0
	var firstURL string
	for i, e := range h.Log.Entries {
		if !strings.Contains(e.Request.URL, "ninebot.com") {
			continue
		}
		body := strings.TrimSpace(e.Response.Content.Text)
		var env map[string]json.RawMessage
		if !strings.HasPrefix(body, "{") || json.Unmarshal([]byte(body), &env) != nil {
			continue
		}
		sRaw, okS := env["s"]
		rRaw, okR := env["r"]
		if !okS || !okR {
			continue
		}
		var sB64, rB64 string
		_ = json.Unmarshal(sRaw, &sB64)
		_ = json.Unmarshal(rRaw, &rB64)
		sig, err := base64.StdEncoding.DecodeString(sB64)
		if err != nil {
			continue
		}
		rBytes, _ := base64.StdEncoding.DecodeString(rB64)
		tried++
		if firstURL == "" {
			firstURL = fmt.Sprintf("[%d] %s", i, e.Request.URL)
		}
		rInputs := []struct {
			name string
			data []byte
		}{
			{"r-decoded", rBytes},
			{"r-b64str", []byte(rB64)},
		}
		hashDefs := []struct {
			hname string
			sum   func([]byte) []byte
			ch    crypto.Hash
		}{
			{"md5", func(b []byte) []byte { s := md5.Sum(b); return s[:] }, crypto.MD5},
			{"sha1", func(b []byte) []byte { s := sha1.Sum(b); return s[:] }, crypto.SHA1},
			{"sha256", func(b []byte) []byte { s := sha256.Sum256(b); return s[:] }, crypto.SHA256},
		}
	done:
		for _, hi := range hashDefs {
			for _, ri := range rInputs {
				digest := hi.sum(ri.data)
				if err := rsa.VerifyPKCS1v15(rsaPub, hi.ch, digest, sig); err == nil {
					fmt.Printf("  VERIFIED [%d] %s: s = RSA-sig(%s over %s)\n", i, e.Request.URL, hi.hname, ri.name)
					verified++
					break done
				}
			}
		}
	}
	fmt.Printf("  -> %d/%d encrypted responses verified (sample: %s)\n", verified, tried, firstURL)

	fmt.Println("\n=== [2] sign header probing (entries with sign+timestamp) ===")
	for i, e := range h.Log.Entries {
		if !strings.Contains(e.Request.URL, "api-passport-bj.ninebot.com") {
			continue
		}
		var sign, ts, av, osv, osl, clientID, osVer string
		var bodyLen int
		for _, hd := range e.Request.Headers {
			switch strings.ToLower(hd.Name) {
			case "sign":
				sign = hd.Value
			case "timestamp":
				ts = hd.Value
			case "app_version":
				av = hd.Value
			case "os":
				osv = hd.Value
			case "os_language":
				osl = hd.Value
			case "os_version":
				osVer = hd.Value
			case "clientid":
				clientID = hd.Value
			}
		}
		if osVer == "" {
			osVer = "27.0"
		}
		if e.Request.PostData != nil {
			bodyLen = len(e.Request.PostData.Text)
		}
		if sign == "" || ts == "" {
			continue
		}
		u := e.Request.URL
		if p := strings.Index(u, "ninebot.com"); p >= 0 {
			u = u[p+len("ninebot.com"):]
		}
		fmt.Printf("  entry[%d] %s sign=%s... ts=%s app_version=%s os=%s/%s body=%dB\n", i, u, sign[:16], ts, av, osv, osl, bodyLen)
		// c1 runs the project's own crypto implementation — verifying the
		// shipped code path, not a copy of the algorithm.
		c1params := map[string]string{
			"app_version": av, "os": osv, "os_language": osl, "os_version": osVer,
		}
		c1canon := nbcrypto.PassportCanonicalString(u, c1params, ts)
		c1got := nbcrypto.PassportSign(c1canon)
		mark := "    "
		if c1got == sign {
			mark = "MATCH"
		}
		fmt.Printf("  %s %-26s sha256=%s...\n", mark, "c1 project-impl", c1got[:24])
		if mark == "MATCH" {
			fmt.Printf("  CANONICAL: %s\n", c1canon)
		}
		cands := map[string]map[string]string{
			"c2 no-clientKey": {
				"url": u, "timestamp": ts,
				"app_version": av, "os": osv, "os_language": osl, "os_version": osVer,
			},
			"c3 clientId-as-key": {
				"clientId": clientID, "url": u, "timestamp": ts,
				"app_version": av, "os": osv, "os_language": osl, "os_version": osVer,
			},
			"c4 with-body-len": {
				"clientKey": harVerifyClientKey, "url": u, "timestamp": ts, "body": fmt.Sprint(bodyLen),
				"app_version": av, "os": osv, "os_language": osl, "os_version": osVer,
			},
		}
		names := make([]string, 0, len(cands))
		for n := range cands {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, name := range names {
			canon := harCanonical(cands[name])
			sum := sha256.Sum256([]byte(canon))
			got := hex.EncodeToString(sum[:])
			mark := "    "
			if got == sign {
				mark = "MATCH"
			}
			fmt.Printf("  %s %-26s sha256=%s...\n", mark, name, got[:24])
			if mark == "MATCH" {
				fmt.Printf("  CANONICAL: %s\n", canon)
			}
		}
	}

	fmt.Println("\n=== [3] request envelope field order / value types ===")
	type stat struct {
		order string
		types string
		n     int
	}
	stats := map[string]*stat{}
	for _, e := range h.Log.Entries {
		if !strings.Contains(e.Request.URL, "ninebot.com") || e.Request.PostData == nil {
			continue
		}
		t := strings.TrimSpace(e.Request.PostData.Text)
		if !strings.HasPrefix(t, "{\"") {
			continue
		}
		keys, vals, err := harTopLevelObject(t)
		if err != nil || len(keys) == 0 {
			continue
		}
		hasK := false
		for _, k := range keys {
			if k == "k" {
				hasK = true
			}
		}
		if !hasK {
			continue
		}
		types := make([]string, 0, len(keys))
		for i, k := range keys {
			if strings.HasPrefix(strings.TrimSpace(vals[i]), "\"") {
				types = append(types, k+":str")
			} else {
				types = append(types, k+":num")
			}
		}
		id := harHostOf(e.Request.URL) + "|" + strings.Join(keys, ",")
		if stats[id] == nil {
			stats[id] = &stat{order: strings.Join(keys, ","), types: strings.Join(types, " ")}
		}
		stats[id].n++
	}
	ids := make([]string, 0, len(stats))
	for id := range stats {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		s := stats[id]
		parts := strings.SplitN(id, "|", 2)
		fmt.Printf("  x%-4d %-28s order: %s\n", s.n, parts[0], s.order)
		fmt.Printf("       types: %s\n", s.types)
	}
	return nil
}

func harCanonical(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, "&")
}

func harHostOf(u string) string {
	u = strings.TrimPrefix(u, "https://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	return u
}

// harTopLevelObject returns the top-level key order and raw values of a
// flat JSON object using the streaming decoder (order-preserving).
func harTopLevelObject(s string) ([]string, []string, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil, fmt.Errorf("not an object")
	}
	var keys, vals []string
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, _ := kt.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, err
		}
		keys = append(keys, key)
		vals = append(vals, string(raw))
	}
	if _, err := dec.Token(); err != nil { // closing '}'
		return nil, nil, err
	}
	return keys, vals, nil
}

// harVerifyPubPEM mirrors crypto's embedded passport public key.
const harVerifyPubPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC6maFY3dEhgav1147RW2gVWzCv
agkiRySnCDRSTM67YhHvLcrUSMnngxJl0A2liFLJydpn65E58oh0Phtu+t4Kkkfe
GIHsr931wRMRtkila4F/RF3U5pqSt42k/10U087QEhGMGvdOzF/5ziGXJod6ovBx
yk6pJlzNhxLTVJSzkQIDAQAB
-----END PUBLIC KEY-----`

// harVerifyClientKey mirrors crypto.PassportClientKey.
const harVerifyClientKey = "e177176a-3b3e-1513-e26e-d1123034cb66"
