package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Logger struct {
	Prefix string
}

func (log *Logger) Init(prefix string) {
	log.Prefix = prefix
}

func (log Logger) Input(query string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(query)
	a, _ := reader.ReadString('\n')
	return strings.TrimRight(a, "\r\n")
}

func (log Logger) PInput(query string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("[" + log.Prefix + "] " + query)
	a, _ := reader.ReadString('\n')
	return strings.TrimRight(a, "\r\n")
}

func (log Logger) Print(query string) {
	fmt.Print(query)
}

func (log Logger) Println(query string) {
	fmt.Println(query)
}

func (log Logger) PPrint(query string) {
	fmt.Print("[" + log.Prefix + "] " + query)
}

func (log Logger) PPrintln(query string) {
	fmt.Println("[" + log.Prefix + "] " + query)
}

func (log Logger) DrawLogo() {
	log.Println("``_____`_`_`````````______`_`_`````````````\n`|``__`(_|_)````````|``___(_)`|````````````\n`|`|``\\/_`_`______`_|`|_```_|`|`___`_`__```\n`|`|`__|`|`|_``/`_``|``_|`|`|`|/`_`\\`'__|``\n`|`|_\\`\\`|`|/`/`(_|`|`|```|`|`|``__/`|`````\n``\\____/_|`/___\\__,_\\_|```|_|_|\\___|_|`````\n````````_/`|```````````````````````````````\n```````|__/````````````````````````````````\nCreated by Crocoby https://cutt.ly/6wo69XUa\n")
}
