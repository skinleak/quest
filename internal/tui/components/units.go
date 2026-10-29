package components

import (
	"strconv"
	"time"
)

// FormatDuration renders d with two to three significant digits: "0.42 ms",
// "8.3 ms", "143 ms", "1.24 s".
func FormatDuration(d time.Duration) string {
	ms := float64(d) / float64(time.Millisecond)
	// The bounds sit below the next unit so rounding never prints "10.0 ms"
	// or "1000 ms".
	switch {
	case ms < 0.995:
		return strconv.FormatFloat(ms, 'f', 2, 64) + " ms"
	case ms < 9.95:
		return strconv.FormatFloat(ms, 'f', 1, 64) + " ms"
	case ms < 999.5:
		return strconv.FormatFloat(ms, 'f', 0, 64) + " ms"
	}
	return strconv.FormatFloat(ms/1000, 'f', 2, 64) + " s"
}

// FormatSize renders a byte count: "512 B", "2.1 KB", "3.4 MB".
func FormatSize(n int64) string {
	switch {
	case n < 1<<10:
		return strconv.FormatInt(n, 10) + " B"
	case n < 1<<20:
		return strconv.FormatFloat(float64(n)/(1<<10), 'f', 1, 64) + " KB"
	}
	return strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64) + " MB"
}
