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
	Tasks map[string]*url.URL
}
type ServiceDiscovery struct {
	ServiceName                         string
	ClusterName                         string
	ECSClient                           *ecs.Client
	EC2Client                           *ec2.Client
	once                                sync.Once
	containerInstanceArnToEC2InstanceId Cache
	ec2InstancesToAddress               Cache
}

func (sd *ServiceDiscovery) initSD() {
	sd.once.Do(func() {
		sd.containerInstanceArnToEC2InstanceId = NewCache(512)
		sd.ec2InstancesToAddress = NewCache(512)
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

	/*containerInstanceArnToEC2InstanceId := make(map[string]string)
	for _, task := range taskDetails.Tasks {
		containerInstanceArnToEC2InstanceId[*task.ContainerInstanceArn] = ""
		if s, ok := sd.containerInstanceArnToEC2InstanceId.Get(*task.ContainerInstanceArn); ok {
			containerInstanceArnToEC2InstanceId[*task.ContainerInstanceArn] = s
		}
	}*/

	var unknownInstanceMap = map[string]string{}
	for _, task := range taskDetails.Tasks {
		if task.ContainerInstanceArn == nil {
			continue
		}
		if _, ok := sd.containerInstanceArnToEC2InstanceId.Get(*task.ContainerInstanceArn); !ok {
			unknownInstanceMap[*task.ContainerInstanceArn] = ""
		}
	}
	var unknownInstanceList []string
	for k, _ := range unknownInstanceMap {
		unknownInstanceList = append(unknownInstanceList, k)
	}
	if len(unknownInstanceList) > 0 {
		ecsInstanceDetails, err := sd.ECSClient.DescribeContainerInstances(context.Background(),
			&ecs.DescribeContainerInstancesInput{
				ContainerInstances: unknownInstanceList,
				Cluster:            &sd.ClusterName,
			})
		if err != nil {
			return nil, err
		}
		if len(ecsInstanceDetails.ContainerInstances) < 1 {
			return nil, fmt.Errorf("no container instances returned")
		}

		for _, instance := range ecsInstanceDetails.ContainerInstances {
			sd.containerInstanceArnToEC2InstanceId.Put(*instance.ContainerInstanceArn, *instance.Ec2InstanceId)
		}
	}
	var unknownEc2 []string
	for _, task := range taskDetails.Tasks {
		arn := *task.ContainerInstanceArn
		ec2Id, ok := sd.containerInstanceArnToEC2InstanceId.Get(arn)
		if !ok {
			log.Println("error. skipping id:  ", ec2Id)
			continue
		}
		if _, ok := sd.ec2InstancesToAddress.Get(ec2Id); !ok {
			unknownEc2 = append(unknownEc2, ec2Id)
		}
	}
	if len(unknownEc2) > 0 {
		ec2ClientDetails, err := sd.EC2Client.DescribeInstances(context.Background(), &ec2.DescribeInstancesInput{
			InstanceIds: unknownEc2,
		})
		if err != nil {
			return nil, err
		}
		if len(ec2ClientDetails.Reservations) < 1 {
			return nil, fmt.Errorf("no ec2 reservations found")
		}

		for _, reservation := range ec2ClientDetails.Reservations {
			for _, instance := range reservation.Instances {
				sd.ec2InstancesToAddress.Put(*instance.InstanceId, *instance.PrivateIpAddress)
			}
		}
	}

	services := &ServiceMap{}
	services.Tasks = make(map[string]*url.URL, len(taskDetails.Tasks))
	for _, task := range taskDetails.Tasks {
		if task.LastStatus == nil || len(task.Containers) < 1 || len(task.Containers[0].NetworkBindings) < 1 {
			continue
		}
		nb := task.Containers[0].NetworkBindings[0]
		ip := "<nil>"
		port := "<nil>"
		if ec2Id, ok := sd.containerInstanceArnToEC2InstanceId.Get(*task.ContainerInstanceArn); ok {
			ip = "<resolvedEC2>"
			if addr, ok := sd.ec2InstancesToAddress.Get(ec2Id); ok {
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
