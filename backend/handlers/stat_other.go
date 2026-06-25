//go:build !unix

package handlers

import "os"

// statCreationMillis has no portable implementation off Unix; callers fall
// back to ModTime. Present so the package builds/tests on any OS.
func statCreationMillis(info os.FileInfo) (int64, bool) {
	return 0, false
}
