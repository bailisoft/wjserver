package data

import (
	"bufio"
	"lxsoft/amwj/logx"
	"os"
	"strings"
)

var BackerChan chan string
var ShopChan chan string
var LabelChan chan string
var ChainChan chan string

func init() {
	BackerChan = make(chan string, 3)
	ShopChan = make(chan string, 3)
	LabelChan = make(chan string, 3)
	ChainChan = make(chan string, 9)
}

func SaveFileLine(filePath string, line string) {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0664)
	if err != nil {
		logx.Fatalf("save to %s failed: %v\n", filePath, err)
		return
	}
	w := bufio.NewWriter(file)
	w.WriteString(line)
	if !strings.HasSuffix(line, "\n") {
		w.WriteString("\n")
	}
	w.Flush()
	file.Close()
}
