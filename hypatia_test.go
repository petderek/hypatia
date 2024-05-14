package hypatia

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"testing"
)

func TestServe(t *testing.T) {
	tpClient := &TaskProtectionStub{
		Protection: &Protection{
			TaskArn: aws.String("arn:aws:ecs:us-west-2:0123456789:task/foo"),
		},
	}
	srv := &Server{
		Protection:   tpClient,
		LocalHealth:  FileHealthcheck{Filepath: "local.status"},
		RemoteHealth: FileHealthcheck{Filepath: "remote.status"},
		Writeable:    true,
	}
	println(srv)
}
