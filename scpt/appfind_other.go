//go:build !darwin

package scpt

// appInstalled reports whether an alias record's application is installed,
// which only macOS can tell.
func appInstalled(rec []byte, file string) bool {
	return false
}
