package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/petderek/hypatia"
	"os"
)

func main() {
	flagMinutes := flag.Int("minutes", 0, "number of minutes to be protected")
	flag.Parse()
	signal := flag.Arg(0)
	tp := &hypatia.TaskProtectionClient{}
	var protection *hypatia.Protection
	var err error

	switch signal {
	case "on":
		var minutes *int
		if *flagMinutes != 0 {
			minutes = flagMinutes
		}
		protection, err = tp.Put(&hypatia.TaskProtectionRequest{
			ProtectionEnabled: aws.Bool(false),
			ExpiresInMinutes:  minutes,
		})
	case "off":
		protection, err = tp.Put(&hypatia.TaskProtectionRequest{
			ProtectionEnabled: aws.Bool(false),
		})
	default:
		protection, err = tp.Get()
	}

	if err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)
	}
	fmt.Println("success: ", *protection.ProtectionEnabled, *protection.ExpirationDate, *protection.TaskArn)
}
