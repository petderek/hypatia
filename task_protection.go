package hypatia

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
)

type TaskProtection interface {
	Get() (*Protection, error)
	Put(enabled bool, minutes *int) (*Protection, error)
}

type TaskProtectionClient struct {
	Location *url.URL
	Client   *http.Client
	_once    sync.Once
}

func (tpc *TaskProtectionClient) init() error {
	var err error
	tpc._once.Do(func() {
		if tpc.Client == nil {
			tpc.Client = http.DefaultClient
		}
		if tpc.Location == nil {
			root, ok := os.LookupEnv("ECS_AGENT_URI")
			if !ok {
				err = errors.New("no ECS_AGENT_URI found")
				return
			}
			tpc.Location, err = url.Parse(root + "/task-protection/v1/state")
			if err != nil {
				return
			}
		}
	})
	return err
}
func (tpc *TaskProtectionClient) Get() (*Protection, error) {
	return tpc.doRequest(http.MethodGet, nil)
}

func (tpc *TaskProtectionClient) Put(request *TaskProtectionRequest) (*Protection, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return tpc.doRequest(http.MethodPut, bytes.NewReader(body))
}

func (tpc *TaskProtectionClient) doRequest(method string, body io.Reader) (*Protection, error) {
	if err := tpc.init(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, tpc.Location.String(), body)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPut {
		req.Header.Add("Content-Type", "application/json")
	}
	res, err := tpc.Client.Do(req)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var tpr TaskProtectionResponse
	err = json.Unmarshal(raw, &tpr)
	if err != nil {
		return nil, err
	}
	if tpr.Protection == nil && tpr.Protection.TaskArn != nil && *tpr.Protection.TaskArn != "" {
		return tpr.Protection, nil
	}
	return nil, errors.New("unable to decipher response: " + string(raw))
}

type TaskProtectionResponse struct {
	RequestID  *string
	Protection *Protection
	Failure    *TaskProtectionFailure
	Error      *TaskProtectionError
}

type TaskProtectionRequest struct {
	ProtectionEnabled *bool `json:"ProtectionEnabled,omitempty"`
	ExpiresInMinutes  *int  `json:"ExpiresInMinutes,omitempty"`
}

type Protection struct {
	ExpirationDate    *string
	ProtectionEnabled *bool
	TaskArn           *string
}

type TaskProtectionFailure struct {
	Arn    *string
	Detail *string
	Reason *string
}

type TaskProtectionError struct {
	Arn     *string
	Code    *string
	Message *string
}
