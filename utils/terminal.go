package utils

import (
	"os"
	"os/exec"
	"runtime"
)

func runCmd(name string, arg ...string) {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func ClearTerminal() {
	if runtime.GOOS == "windows" {
		runCmd("cmd", "/c", "cls")
	} else {
		runCmd("clear")
	}
}
