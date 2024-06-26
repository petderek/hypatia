package hypatia

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

type TaskProtectionClient struct {
	Location  *url.URL
	Client    *http.Client
	_once     sync.Once
	_memo     sync.Once
	_metadata *TaskMetadata
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

func (tpc *TaskProtectionClient) Put(protect bool, min *int) (*Protection, error) {
	request := &TaskProtectionRequest{
		ProtectionEnabled: &protect,
		ExpiresInMinutes:  min,
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	log.Println("req body: ", string(body))
	return tpc.doRequest(http.MethodPut, bytes.NewReader(body))
}

func (tpc *TaskProtectionClient) Self() (*TaskMetadata, error) {
	var metadata TaskMetadata
	if tpc._metadata != nil {
		metadata = *tpc._metadata
		return &metadata, nil
	}
	if err := tpc.init(); err != nil {
		return nil, err
	}
	location, ok := os.LookupEnv("ECS_CONTAINER_METADATA_URI_V4")
	if !ok {
		return nil, errors.New("no ECS_CONTAINER_METADATA_URI_V4 set")
	}
	u, err := url.Parse(location + "/task")
	if err != nil {
		return nil, err
	}
	r, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	tpc._memo.Do(func() {
		tpc._metadata = &metadata
	})
	return &metadata, nil
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
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	log.Println("res body: ", string(raw))
	var tpr TaskProtectionResponse
	err = json.Unmarshal(raw, &tpr)
	if err != nil {
		return nil, err
	}
	if tpr.Protection != nil && tpr.Protection.TaskArn != nil && *tpr.Protection.TaskArn != "" {
		return tpr.Protection, nil
	}
	return nil, errors.New("unable to decipher response: " + string(raw))
}

type TaskProtectionResponse struct {
	RequestID  *string                `json:"requestID,omitempty"`
	Protection *Protection            `json:"protection,omitempty"`
	Failure    *TaskProtectionFailure `json:"failure,omitempty"`
	Error      *TaskProtectionError   `json:"error,omitempty"`
}

type TaskProtectionRequest struct {
	ProtectionEnabled *bool `json:"ProtectionEnabled,omitempty"`
	ExpiresInMinutes  *int  `json:"ExpiresInMinutes,omitempty"`
}

type TaskMetadata struct {
	TaskARN       *string
	Cluster       *string
	EC2InstanceId *string
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

type TaskProtectionStub struct {
	*Protection
}

func (t *TaskProtectionStub) Get() (*Protection, error) {
	return t.Protection, nil
}

func (t *TaskProtectionStub) Self() (*TaskMetadata, error) {
	return nil, errors.New("metadata not implemented")
}
func (t *TaskProtectionStub) Put(enabled bool, minutes *int) (*Protection, error) {
	t.ProtectionEnabled = &enabled
	if minutes != nil && *minutes > 0 {
		next := time.Now().Add(time.Minute * time.Duration(*minutes)).String()
		t.ExpirationDate = &next
	}
	return t.Protection, nil
}
