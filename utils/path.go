package utils

import "os"

// Returns whether a file or folder according to the filter exists
func ExistsDirOrFile(any bool, dir bool, path string) bool {
	inf, err := os.Stat(path)
	if err != nil {
		return false
	} else if any {
		return true
	} else if dir {
		return inf.IsDir()
	} else {
		return !inf.IsDir()
	}
}
