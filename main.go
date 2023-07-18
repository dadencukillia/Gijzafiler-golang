package main

import (
	"GijzaFiler/client"
	"GijzaFiler/utils"
)

func main() {
	utils.ClearTerminal()
	logologger := utils.Logger{Prefix: ""}
	logologger.DrawLogo()
	client.StarterMenu()
}
