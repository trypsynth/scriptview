package scpt

import "github.com/trypsynth/scriptview/classic"

// Before Mac OS X, a compiled script kept its data in the file's resource
// fork, as a 'scpt' resource, and left the data fork empty. unwrapScript
// finds the compiled script in a wrapper that carries the resource fork
// (AppleSingle, AppleDouble, MacBinary, or BinHex), or in a bare resource
// fork.
func unwrapScript(data []byte) ([]byte, bool) {
	if f, ok := classic.Unwrap(data); ok {
		if fasdHeaderLen(f.Data) > 0 {
			return f.Data, true
		}
		return resourceScript(f.Rsrc)
	}
	return resourceScript(data)
}

// resourceScript returns the 'scpt' resource of a resource fork, preferring
// ID 128, which Script Editor used.
func resourceScript(fork []byte) ([]byte, bool) {
	res := classic.Resources(fork, "scpt")
	if s, ok := res[128]; ok && fasdHeaderLen(s) > 0 {
		return s, true
	}
	for _, s := range res {
		if fasdHeaderLen(s) > 0 {
			return s, true
		}
	}
	return nil, false
}
