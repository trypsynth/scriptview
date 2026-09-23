package scpt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	appSearchMu    sync.Mutex
	appSearchCache = map[string]bool{}
)

// appInstalled reports whether the application an alias record names is on
// this Mac: at the alias's path, or elsewhere under the same file name, as
// Launch Services would find it.
func appInstalled(rec []byte, file string) bool {
	if path, ok := aliasTag(rec, aliasTagPOSIXPath); ok {
		mount, ok := aliasTag(rec, aliasTagMountPoint)
		if !ok {
			mount = []byte("/")
		}
		if _, err := os.Stat(filepath.Join(string(mount), string(path))); err == nil {
			return true
		}
	}
	if !strings.HasSuffix(file, ".app") || strings.ContainsAny(file, `'"\`) {
		return false
	}
	appSearchMu.Lock()
	defer appSearchMu.Unlock()
	found, ok := appSearchCache[file]
	if !ok {
		out, err := exec.Command("mdfind", "kMDItemContentType == 'com.apple.application-bundle' && kMDItemFSName == '"+file+"'").Output()
		found = err == nil && len(strings.TrimSpace(string(out))) > 0
		appSearchCache[file] = found
	}
	return found
}
