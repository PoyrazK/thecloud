package safehelper

import "fmt"

// SafeByte converts int to byte, panicking if out of range [0,255].
// Callers must guarantee the value is in range.
// This satisfies gosec G115.
func SafeByte(v int) byte {
	if v < 0 || v > 255 {
		panic(fmt.Sprintf("SafeByte: value %d out of range", v))
	}
	return byte(v)
}

// SafeInt64 converts uint64 to int64, panicking if out of range.
// Callers must guarantee the value is within int64 range.
// This satisfies gosec G115.
func SafeInt64(v uint64) int64 {
	if v > 1<<63-1 {
		panic(fmt.Sprintf("SafeInt64: value %d out of range", v))
	}
	return int64(v)
}
