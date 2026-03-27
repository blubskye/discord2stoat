package stoat

import "fmt"

// intToCSS converts a Discord integer color (e.g. 0xFF5733) to a CSS hex string ("#FF5733").
func intToCSS(color int) string {
	return fmt.Sprintf("#%06X", color&0xFFFFFF)
}
