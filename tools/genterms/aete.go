//go:build darwin

package main

import (
	"os"

	"github.com/trypsynth/scriptview/internal/aete"
)

// appleScriptRsrc holds AppleScript's own built-in terminology as an 'aeut'
// resource (classic aete format) in a data-fork resource file.
const appleScriptRsrc = "/System/Library/Components/AppleScript.component/Contents/Resources/AppleScript.rsrc"

// readResource returns resource (typ, id) from a data-fork resource file.
func readResource(path, typ string, id int16) ([]byte, error) {
	d, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return aete.Resource(d, typ, id)
}

// parseAete decodes an aete/aeut resource into terms for scope.
func parseAete(d []byte, scope string) ([]term, error) {
	ts, err := aete.Parse(d, scope)
	out := make([]term, len(ts))
	for i, t := range ts {
		out[i] = term{scope: t.Scope, kind: t.Kind, code: t.Code, name: t.Name, plural: t.Plural}
	}
	return out, err
}
