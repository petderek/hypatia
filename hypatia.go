package hypatia

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"io"
	"log"
	"net/http"
	"strings"
)

type TaskProtectionIface interface {
	Get() (*Protection, error)
	Put(enabled bool, minutes *int) (*Protection, error)
}

type Server struct {
	Protection       TaskProtectionIface
	LocalHealth      FileHealthcheck
	RemoteHealth     FileHealthcheck
	ServiceDiscovery *ServiceDiscovery
	Writeable        bool
}

type Neighbor struct {
	TaskArn *string `json:"taskArn,omitempty"`
	Address *string `json:"address,omitempty"`
}
type RequestResponse struct {
	TaskArn               *string    `json:"taskArn,omitempty"`
	TaskProtectionEnabled *bool      `json:"taskProtectionEnabled,omitempty"`
	TaskProtectionExpiry  *string    `json:"taskProtectionExpiry,omitempty"`
	SetLocalHealth        *bool      `json:"setLocalHealth,omitempty"`
	SetRemoteHealth       *bool      `json:"setRemoteHealth,omitempty"`
	LocalHealth           *string    `json:"localHealth,omitempty"`
	RemoteHealth          *string    `json:"remoteHealth,omitempty"`
	ExpiresInMinutes      *int       `json:"expiresInMinutes,omitempty"`
	Neighbors             []Neighbor `json:"neighbors,omitempty"`
	Errors                []string   `json:"errors,omitempty"`
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
	for k, v := range services.Tasks {
		output.Neighbors = append(output.Neighbors, Neighbor{
			TaskArn: aws.String(k),
			Address: aws.String(v.String()),
		})
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
	if isPing(req) {
		hs.ServePing(res, req)
		return
	}

	if isNeighbors(req) {
		hs.ServeNeighbors(res, req)
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
			}
		}
		if input.SetRemoteHealth != nil {
			if err := hs.RemoteHealth.SetHealth(*input.SetRemoteHealth); err != nil {
				errors = append(errors, err)
			}
		}
		if input.SetLocalHealth != nil {
			if err := hs.LocalHealth.SetHealth(*input.SetLocalHealth); err != nil {
				errors = append(errors, err)
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

func isNeighbors(req *http.Request) bool {
	return strings.EqualFold(req.URL.Path, "/neighbors")
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
