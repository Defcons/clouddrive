//go:build !unix

package handlers

// fsSpace has no portable implementation off Unix; total/free are reported as
// 0. Present so the package builds/tests on any OS.
func fsSpace(path string) (total, free int64) {
	return 0, 0
}
