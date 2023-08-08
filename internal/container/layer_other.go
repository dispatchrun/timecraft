//go:build !unix

package container

import "os"

func duplicateFile(f *os.File, name string) (*os.File, error) {
	// Technically we could return the temporary file since it points to the
	// same layer, however:
	//
	// - The value returned by calling Name() on the *os.File is the string
	//   that was used to open it, and the temporary file was opened with a
	//   different name than the final layer file path.
	//
	// - The temporary file is open for writing, the program could mistakenly
	//   alter the content of the layer if it were to write to it. Reopening
	//   the file in read-only mode is an extra safeguard that could prevent
	//   issues in the future.
	//
	// Note that this is racy, a concurrent deletion of the file could cause
	// the reopen to fail. This is why we only use this implementation as a
	// fallback when dup(2) isn't available.
	return os.Open(name)
}
