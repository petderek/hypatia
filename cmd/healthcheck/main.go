package main

import (
	"flag"
	"fmt"
	"github.com/petderek/hypatia"
	"os"
)

func main() {
	file := flag.String("file", "", "number of minutes to be protected")
	flag.Parse()
	signal := flag.Arg(0)
	hc := &hypatia.FileHealthcheck{Filepath: *file}

	var err error
	switch signal {
	case "on":
		err = hc.SetHealth(true)
	case "off":
		err = hc.SetHealth(false)
	default:
		err = hc.GetHealth()
	}
	if err != nil {
		fmt.Println("failed: ", err)
		os.Exit(1)
	}
	fmt.Println("succeeded")
}
