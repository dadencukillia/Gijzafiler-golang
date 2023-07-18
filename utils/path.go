package utils

import "os"

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
