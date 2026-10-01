package crypto

// ControlURL maps an operation short form to its control endpoint
// (open_buck uses the long URL form while the operation field keeps
// the short form, mirroring the reference CLI).
func ControlURL(op string) string {
	switch op {
	case "bell":
		return "/devices/control/bell"
	case "engine_start":
		return "/devices/control/engine_start"
	case "engine_stop":
		return "/devices/control/engine_stop"
	case "buck":
		return "/devices/control/open_buck"
	}
	return "/devices/control/" + op
}
