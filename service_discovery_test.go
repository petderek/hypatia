package hypatia

import (
	"testing"
)

func TestServiceDiscovery(t *testing.T) {
	sd := ServiceDiscovery{
		ServiceName: "hypatia",
		ClusterName: "default",
	}
	things, err := sd.GetServiceMap()
	t.Log("yay")
	t.Log(things)
	t.Log(err)
}
