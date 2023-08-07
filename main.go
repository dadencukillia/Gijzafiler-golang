package main

import (
	"GijzaFiler/client"
	"GijzaFiler/server"
	"GijzaFiler/utils"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) > 1 {
		if os.Args[1] == "cl" || os.Args[1] == "client" || os.Args[1] == "c" {
			if len(os.Args) == 2 {
				cl := client.Create(client.CollectClientData())
				cl.Run()
			} else {
				cl := client.Create(client.GetPortAndIp(strings.Join(os.Args[2:], " ")))
				cl.Run()
			}
		} else if os.Args[1] == "srv" || os.Args[1] == "server" || os.Args[1] == "s" {
			if len(os.Args) == 2 {
				serv := server.Create(server.CollectServerData())
				serv.Run()
			} else {
				params := os.Args[2:]
				dirname := strings.Join(params, " ")
				encrypt := false

				if params[0] == "-e" && len(params) > 1 {
					dirname = strings.Join(params[1:], " ")
					encrypt = true
				}
				if utils.ExistsDirOrFile(false, true, dirname) {
					serv := server.Create(5416, dirname, encrypt, []string{}, -1)
					serv.Run()
				} else {
					fmt.Println("Incorrect arguments, scheme:\n• GijzaFiler server [-e] {directory path}\nThe \"e\" option enables E2E encryption")
				}
			}
		} else if os.Args[1] == "ui" || os.Args[1] == "interface" || os.Args[1] == "i" {
			client.StarterMenu()
		} else {
			fmt.Println("You use launch GijzaFiler from the console. You have entered an unknown mode. Available modes:\n• GijzaFiler client\n• GijzaFiler server\n• GijzaFiler interface")
		}
		return
	}
	utils.ClearTerminal()
	logologger := utils.Logger{Prefix: ""}
	logologger.DrawLogo()
	client.StarterMenu()
}
