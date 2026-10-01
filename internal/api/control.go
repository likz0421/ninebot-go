package api

// ControlOpInfo pairs the short operation name with its endpoint and
// safety classification (mirrors the reference CLI's controlOpInfo).
type ControlOpInfo struct {
	Op      string // short form used in the body ("operation" field)
	URL     string // endpoint (long form for buck)
	Safety  string // "prompt" | "safe"
	Warning string
}

var controlOps = map[string]ControlOpInfo{
	"bell": {
		Op:     "bell",
		URL:    "/devices/control/bell",
		Safety: "safe",
	},
	"engine_start": {
		Op:      "engine_start",
		URL:     "/devices/control/engine_start",
		Safety:  "prompt",
		Warning: "ACTUATES the physical vehicle. Use only with the owner's consent while physically present.",
	},
	"engine_stop": {
		Op:      "engine_stop",
		URL:     "/devices/control/engine_stop",
		Safety:  "prompt",
		Warning: "ACTUATES the physical vehicle. Use only with the owner's consent while physically present.",
	},
	"buck": {
		Op:      "buck",
		URL:     "/devices/control/open_buck",
		Safety:  "prompt",
		Warning: "ACTUATES the physical vehicle. Use only with the owner's consent while physically present.",
	},
}

// LookupControlOp resolves an op short form.
func LookupControlOp(op string) (ControlOpInfo, bool) {
	oi, ok := controlOps[op]
	return oi, ok
}
