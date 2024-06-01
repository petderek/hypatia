package hypatia

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
)

type TaskProtectionIface interface {
	Get() (*Protection, error)
	Put(enabled bool, minutes *int) (*Protection, error)
	TaskMetadataIface
}

type TaskMetadataIface interface {
	Self() (*TaskMetadata, error)
}

type Server struct {
	Protection       TaskProtectionIface
	Metadata         TaskMetadataIface
	LocalHealth      FileHealthcheck
	RemoteHealth     FileHealthcheck
	ServiceDiscovery *ServiceDiscovery
	Writeable        bool
	proxy            *httputil.ReverseProxy
	imdsClient       *imds.Client
	once             sync.Once
}

type Neighbor struct {
	TaskArn *string `json:"taskArn,omitempty"`
	Address *string `json:"address,omitempty"`
}
type RequestResponse struct {
	TaskArn               *string  `json:"taskArn,omitempty"`
	TaskProtectionEnabled *bool    `json:"taskProtectionEnabled,omitempty"`
	TaskProtectionExpiry  *string  `json:"taskProtectionExpiry,omitempty"`
	SetLocalHealth        *bool    `json:"setLocalHealth,omitempty"`
	SetRemoteHealth       *bool    `json:"setRemoteHealth,omitempty"`
	LocalHealth           *string  `json:"localHealth,omitempty"`
	RemoteHealth          *string  `json:"remoteHealth,omitempty"`
	ExpiresInMinutes      *int     `json:"expiresInMinutes,omitempty"`
	EC2InstanceId         *string  `json:"ec2Instance,omitempty"`
	Tasks                 []string `json:"tasks,omitempty"`
	Errors                []string `json:"errors,omitempty"`
}

func (hs *Server) initServer() {
	hs.once.Do(func() {
		hs.proxy = &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				if err := hs.doRewrite(req); err != nil {
					log.Println(err)
				}
			},
		}
		if cfg, err := config.LoadDefaultConfig(context.Background()); err == nil {
			hs.imdsClient = imds.NewFromConfig(cfg)
		} else {
			log.Println("error starting imds: ", err)
		}
	})
}

func (hs *Server) ServeProxy(res http.ResponseWriter, req *http.Request) {
	hs.proxy.ServeHTTP(res, req)
}

func (hs *Server) doRewrite(in *http.Request) error {
	taskArn := extractArn(in)
	if taskArn == "" {
		return fmt.Errorf("unable to extract arn: %s", in.URL)
	}
	services, err := hs.ServiceDiscovery.GetServiceMap()
	if err != nil {
		return fmt.Errorf("error getting data from proxy: %s", err)
	}
	addr, ok := services.Tasks[taskArn]
	if !ok {
		return fmt.Errorf("address not found in map: %s", taskArn)
	} else {
		in.URL.Scheme = addr.Scheme
		in.URL.Host = addr.Host
	}
	return nil
}

func (hs *Server) ServePing(res http.ResponseWriter, _ *http.Request) {
	var message []byte
	if err := hs.RemoteHealth.GetHealth(); err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		message = []byte(err.Error())
	} else {
		message = []byte("woof")
	}
	writeResponse(res, message)
}

func (hs *Server) ServeNeighbors(res http.ResponseWriter, _ *http.Request) {
	var output RequestResponse
	if hs.ServiceDiscovery == nil {
		log.Println("no sd configured")
		handleISE(res)
		return
	}
	services, err := hs.ServiceDiscovery.GetServiceMap()
	if err != nil {
		log.Println("unable to get sd data: ", err)
		handleISE(res)
		return
	}
	for k, _ := range services.Tasks {
		output.Tasks = append(output.Tasks, k)
	}
	data, err := json.Marshal(&output)
	if err != nil {
		log.Println("unable to json things: ", err)
		handleISE(res)
		return
	}
	writeResponse(res, data)
}

func (hs *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	hs.initServer()
	if isTasks(req) {
		hs.ServeNeighbors(res, req)
		return
	}

	if hs.isProxy(req) {
		hs.ServeProxy(res, req)
		return
	}

	if isPing(req) {
		hs.ServePing(res, req)
		return
	}

	var errors []error
	var output RequestResponse

	switch req.Method {
	case http.MethodPost:
		if !hs.Writeable {
			log.Println("not authorized for writes")
			handleUnauth(res)
			return
		}
		var input RequestResponse
		processed, err := io.ReadAll(req.Body)
		if err != nil {
			res.WriteHeader(http.StatusBadRequest)
			log.Println("bad request: ", err)
			return
		}
		log.Println("request: ", string(processed))
		err = json.Unmarshal(processed, &input)
		if err != nil {
			res.WriteHeader(http.StatusBadRequest)
			log.Println("bad request: ", err)
			return
		}
		if input.TaskProtectionEnabled != nil {
			if _, err := hs.Protection.Put(*input.TaskProtectionEnabled, input.ExpiresInMinutes); err != nil {
				errors = append(errors, err)
			} else {
				output.TaskProtectionEnabled = input.TaskProtectionEnabled
				output.TaskProtectionEnabled = input.TaskProtectionEnabled
			}
		}
		if input.SetRemoteHealth != nil {
			if err := hs.RemoteHealth.SetHealth(*input.SetRemoteHealth); err != nil {
				errors = append(errors, err)
			} else {
				output.SetRemoteHealth = input.SetRemoteHealth
			}
		}
		if input.SetLocalHealth != nil {
			if err := hs.LocalHealth.SetHealth(*input.SetLocalHealth); err != nil {
				errors = append(errors, err)
			} else {
				output.SetLocalHealth = input.SetLocalHealth
			}
		}
	case http.MethodGet:
		protectionStatus, psErr := hs.Protection.Get()
		if psErr != nil {
			errors = append(errors, psErr)
		} else {
			output.TaskArn = protectionStatus.TaskArn
			output.TaskProtectionExpiry = protectionStatus.ExpirationDate
			output.TaskProtectionEnabled = protectionStatus.ProtectionEnabled
		}

		self, selfErr := hs.Metadata.Self()
		if selfErr != nil {
			errors = append(errors, selfErr)
		} else {
			output.TaskArn = self.TaskARN
		}
		if hs.imdsClient != nil {
			if mt, err := hs.imdsClient.GetInstanceIdentityDocument(context.Background(), &imds.GetInstanceIdentityDocumentInput{}); err == nil {
				log.Println("setting IMDS: ", mt)
				output.EC2InstanceId = aws.String(mt.InstanceID)
			} else {
				errors = append(errors, err)
			}
		} else {
			log.Println("imds is null DELETE ME")
		}

		localHealthStatus := hs.LocalHealth.GetHealth()
		if localHealthStatus != nil {
			errors = append(errors, localHealthStatus)
			output.LocalHealth = aws.String("Unhealthy")
		} else {
			output.LocalHealth = aws.String("Healthy")
		}
		remoteHealthStatus := hs.RemoteHealth.GetHealth()
		if remoteHealthStatus != nil {
			errors = append(errors, remoteHealthStatus)
			output.RemoteHealth = aws.String("Unhealthy")
		} else {
			output.RemoteHealth = aws.String("Healthy")
		}
	default:
		handleUnauth(res)
		return
	}

	if len(errors) > 0 {
		output.Errors = make([]string, len(errors))
		for i, e := range errors {
			output.Errors[i] = e.Error()
		}
	}
	data, err := json.Marshal(&output)
	if err != nil {
		log.Println("error: ", err)
		handleISE(res)
		return
	}
	writeResponse(res, data)
	return
}

func isTasks(req *http.Request) bool {
	return strings.EqualFold(req.URL.Path, "/tasks")
}

func isPing(req *http.Request) bool {
	var found bool
	for _, s := range []string{"/ping", "/ping/"} {
		if strings.EqualFold(req.URL.Path, s) {
			found = true
			break
		}
	}
	return found
}

func extractArn(req *http.Request) string {
	p := req.URL.Path
	if !strings.HasPrefix(p, "/task/") {
		return ""
	}
	tokens := strings.SplitN(p, "/task/", 2)
	if len(tokens) < 2 {
		return ""
	}
	a, err := arn.Parse(tokens[1])
	if err != nil {
		return ""
	}

	if parts := strings.SplitN(a.Resource, "/", 5); len(parts) < 2 {
		return ""
	} else if !strings.EqualFold("task", parts[0]) {
		return ""
	} else if len(parts) > 3 {
		return ""
	}
	return tokens[1]
}

func (hs *Server) isProxy(req *http.Request) bool {
	// 'can you extract a task arn from the request'
	arn := extractArn(req)
	if arn == "" {
		return false
	}

	// 'is it me'
	x, err := hs.Metadata.Self()
	if err != nil {
		panic("error retrieving metadata: " + err.Error())
	}
	if x.TaskARN == nil || *x.TaskARN == arn {
		return false
	}
	return true
}

func handleUnauth(res http.ResponseWriter) {
	res.WriteHeader(http.StatusUnauthorized)
	writeResponse(res, []byte("{}"))
}

func handleISE(res http.ResponseWriter) {
	res.WriteHeader(http.StatusInternalServerError)
	writeResponse(res, []byte("{}"))
}

func writeResponse(res http.ResponseWriter, data []byte) {
	if _, err := res.Write(data); err != nil {
		log.Println("error writing response: ", err)
	}
}
