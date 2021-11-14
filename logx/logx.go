package logx

import (
	"log"
	"os"
)

var (
	DebugMode   bool
	debugLogger = log.New(os.Stdout, "DEBUG: ", log.Lshortfile)
	fatalLogger = log.New(os.Stdout, "FATAL: ", log.Lshortfile)
)

func init() {
	DebugMode = len(os.Args) > 3
}

func Logf(format string, v ...interface{}) {
	if DebugMode {
		debugLogger.Printf(format, v...)
	}
}

func Logln(v ...interface{}) {
	if DebugMode {
		debugLogger.Println(v...)
	}
}

func Fatalf(format string, v ...interface{}) {
	fatalLogger.Fatalf(format, v...)
}

func Fatalln(v ...interface{}) {
	fatalLogger.Fatalln(v...)
}
