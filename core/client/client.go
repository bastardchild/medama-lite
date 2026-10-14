// Package client embeds the frozen, pre-built tracker scripts that are served
// from the MedamaLite core binary.
//
// These artefacts used to be produced by the Bun/Node toolchain in /tracker and
// copied into this directory at build time. MedamaLite freezes them here so that
// no JavaScript runtime is required to build or run the server. To update the
// tracker, rebuild it in a checkout of the upstream tracker and copy the files
// from tracker/dist into ./scripts.
package client

import (
	"embed"
	"io/fs"
)

// Embedded tracker scripts. The files mirror the upstream `tracker/dist`
// directory (both minified and unminified variants) and are served under
// `/scripts/*` exactly as before.
//
//go:embed all:scripts
var trackerFS embed.FS

// Tracker returns the filesystem containing the frozen tracker scripts.
func Tracker() (fs.FS, error) {
	return fs.Sub(trackerFS, "scripts")
}

// ScriptNames lists every embedded tracker script file name.
func ScriptNames() ([]string, error) {
	tracker, err := Tracker()
	if err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(tracker, ".")
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	return names, nil
}
