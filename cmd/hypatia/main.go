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
	serviceName := flag.String("service", "", "the ecs service name to use")
	clusterName := flag.String("cluster", "", "the ecs cluster name to use")
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
	sd := &hypatia.ServiceDiscovery{}
	if *serviceName != "" {
		sd.ServiceName = *serviceName
	}
	if *clusterName != "" {
		sd.ClusterName = *clusterName
	}
	srv := &hypatia.Server{
		Protection:       tpClient,
		Metadata:         tpClient,
		LocalHealth:      hypatia.FileHealthcheck{Filepath: *localfile},
		RemoteHealth:     hypatia.FileHealthcheck{Filepath: *remotefile},
		ServiceDiscovery: sd,
		Writeable:        *writable,
	}
	http.ListenAndServe(*address, srv)
}
