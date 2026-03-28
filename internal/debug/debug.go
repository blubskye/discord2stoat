package debug

import "log"

// Enabled controls whether debug output is emitted.
// Set to true via the --debug flag at startup.
var Enabled bool

// Printf logs a debug message. It is a no-op when Enabled is false.
func Printf(format string, args ...any) {
	if Enabled {
		log.Printf("[DEBUG] "+format, args...)
	}
}
