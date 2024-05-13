package main

import (
	"flag"
	"fmt"
	"github.com/petderek/hypatia"
	"io"
	"log"
	"os"
)

func init() {
	log.SetOutput(io.Discard)
	log.SetFlags(0)
}
func main() {
	flagMinutes := flag.Int("minutes", 0, "number of minutes to be protected")
	verbose := flag.Bool("v", false, "verbose logs")
	flag.Parse()
	if *verbose {
		log.SetOutput(os.Stderr)
	}
	signal := flag.Arg(0)
	log.Printf("using command [%s] with minutes [%d]. 0 minutes uses default\n", signal, flagMinutes)
	tp := &hypatia.TaskProtectionClient{}
	var protection *hypatia.Protection
	var err error

	switch signal {
	case "on":
		var minutes *int
		if *flagMinutes != 0 {
			minutes = flagMinutes
		}
		protection, err = tp.Put(true, minutes)
	case "off":
		protection, err = tp.Put(false, nil)
	default:
		protection, err = tp.Get()
	}

	if err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)
	}
	fmt.Println("success: ", safeB(protection.ProtectionEnabled), safeS(protection.ExpirationDate), safeS(protection.TaskArn))
}

func safeS(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func safeB(b *bool) string {
	if b == nil {
		return "<nil>"
	}
	if *b {
		return "true"
	}
	return "false"
}
