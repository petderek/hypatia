package main

import (
	"flag"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/petderek/hypatia"
	"log"
	"net/http"
)

func main() {
	localfile := flag.String("local", "local.status", "local file healthcheck")
	remotefile := flag.String("remote", "remote.status", "remote file healthcheck")
	address := flag.String("a", ":8000", "address to listen on")
	shouldStub := flag.Bool("stub", false, "should stub task protection endpoint")
	writable := flag.Bool("w", true, "accepts post requests")
	flag.Parse()
	var tpClient hypatia.TaskProtectionIface
	if *shouldStub {
		tpClient = &hypatia.TaskProtectionStub{
			Protection: &hypatia.Protection{
				TaskArn: aws.String("arn:aws:ecs:us-west-2:0123456789:task/foo"),
			},
		}
	} else {
		tpClient = &hypatia.TaskProtectionClient{}
	}
	log.Println("starting server")
	srv := &hypatia.HypatiaServer{
		Protection:   tpClient,
		LocalHealth:  hypatia.FileHealthcheck{Filepath: *localfile},
		RemoteHealth: hypatia.FileHealthcheck{Filepath: *remotefile},
		Writeable:    *writable,
	}
	http.ListenAndServe(*address, srv)
}
