package hypatia

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
)

type ServiceMap struct {
	Tasks              map[string]*url.URL
	ContainerInstances map[string]string
	EC2Instances       map[string]string
}
type ServiceDiscovery struct {
	ServiceName string
	ClusterName string
	ECSClient   *ecs.Client
	EC2Client   *ec2.Client
	once        sync.Once
	cache       Cache
}

func (sd *ServiceDiscovery) initSD() {
	sd.once.Do(func() {
		sd.cache = NewCache(512)
		if sd.ECSClient == nil {
			cfg, err := config.LoadDefaultConfig(context.Background())
			if err != nil {
				panic(err)
			}
			sd.ECSClient = ecs.NewFromConfig(cfg)
		}
		if sd.EC2Client == nil {
			cfg, err := config.LoadDefaultConfig(context.Background())
			if err != nil {
				panic(err)
			}
			sd.EC2Client = ec2.NewFromConfig(cfg)
		}
		if sd.ServiceName == "" {
			root, ok := os.LookupEnv("ECS_CONTAINER_METADATA_URI_V4")
			if !ok {
				panic("no ECS_CONTAINER_METADATA_URI_V4 found, and no service arn provided")
			}
			metadataEndpoint, err := url.Parse(root + "/task")
			if err != nil {
				panic(err)
			}
			resp, err := http.Get(metadataEndpoint.String())
			if err != nil {
				panic("unable to get response from metadata endpoint: " + err.Error())
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}
			var m metadata
			if err := json.Unmarshal(data, &m); err != nil {
				panic("bad json: " + err.Error())
			}
			if m.ServiceName == nil {
				panic("no service name found in response: " + string(data))
			}
			if m.Cluster == nil {
				panic("no cluster name found in response: " + string(data))
			}
			sd.ServiceName = *m.ServiceName
			sd.ClusterName = *m.Cluster
		}
	})
}

func (sd *ServiceDiscovery) GetServiceMap() (*ServiceMap, error) {
	sd.initSD()
	res, err := sd.ECSClient.ListTasks(context.Background(), &ecs.ListTasksInput{
		ServiceName: &sd.ServiceName,
		Cluster:     &sd.ClusterName,
	})
	if err != nil {
		return nil, err
	}
	if len(res.TaskArns) < 1 {
		return nil, fmt.Errorf("no tasks returned")
	}
	target := res.TaskArns
	if len(res.TaskArns) > 100 {
		log.Println("warn: not paginating because developer is lazy")
		target = target[:99]
	}
	taskDetails, err := sd.ECSClient.DescribeTasks(context.Background(), &ecs.DescribeTasksInput{
		Tasks:   target,
		Cluster: &sd.ClusterName,
	})
	if err != nil {
		return nil, err
	}

	containerInstanceArnToEC2InstanceId := make(map[string]string)
	for _, task := range taskDetails.Tasks {
		containerInstanceArnToEC2InstanceId[*task.ContainerInstanceArn] = ""
	}

	var containerInstances []string
	for k, _ := range containerInstanceArnToEC2InstanceId {
		containerInstances = append(containerInstances, k)
	}

	ecsInstanceDetails, err := sd.ECSClient.DescribeContainerInstances(context.Background(),
		&ecs.DescribeContainerInstancesInput{
			ContainerInstances: containerInstances,
			Cluster:            &sd.ClusterName,
		})
	if err != nil {
		return nil, err
	}
	if len(ecsInstanceDetails.ContainerInstances) < 1 {
		return nil, fmt.Errorf("no container instances returned")
	}

	var ec2InstanceIds []string
	for _, instance := range ecsInstanceDetails.ContainerInstances {
		containerInstanceArnToEC2InstanceId[*instance.ContainerInstanceArn] = *instance.Ec2InstanceId
		ec2InstanceIds = append(ec2InstanceIds, *instance.Ec2InstanceId)
	}

	ec2ClientDetails, err := sd.EC2Client.DescribeInstances(context.Background(), &ec2.DescribeInstancesInput{
		InstanceIds: ec2InstanceIds,
	})
	if err != nil {
		return nil, err
	}
	if len(ec2ClientDetails.Reservations) < 1 {
		return nil, fmt.Errorf("no ec2 reservations found")
	}

	ec2InstancesToAddress := make(map[string]string)
	for _, reservation := range ec2ClientDetails.Reservations {
		for _, instance := range reservation.Instances {
			ec2InstancesToAddress[*instance.InstanceId] = *instance.PrivateIpAddress
		}
	}

	services := &ServiceMap{}
	services.Tasks = make(map[string]*url.URL, len(taskDetails.Tasks))
	services.ContainerInstances = containerInstanceArnToEC2InstanceId
	services.EC2Instances = ec2InstancesToAddress
	for _, task := range taskDetails.Tasks {
		if task.LastStatus == nil || *task.LastStatus != "RUNNING" || len(task.Containers) < 1 || len(task.Containers[0].NetworkBindings) < 1 {
			continue
		}
		nb := task.Containers[0].NetworkBindings[0]
		ip := "<nil>"
		port := "<nil>"
		if ec2Id, ok := containerInstanceArnToEC2InstanceId[*task.ContainerInstanceArn]; ok {
			ip = "<resolvedEC2>"
			if addr, ok := ec2InstancesToAddress[ec2Id]; ok {
				ip = addr
			}
		}

		if nb.HostPort != nil {
			port = strconv.Itoa(int(*nb.HostPort))
		}

		u, _ := url.Parse("http://" + ip + ":" + port)
		services.Tasks[*task.TaskArn] = u
	}

	return services, nil
}

type metadata struct {
	ServiceName *string
	Cluster     *string
}
