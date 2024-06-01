package hypatia

import (
	"net/http"
	"testing"
)

func TestArn(t *testing.T) {
	happyCases := []string{
		"http://localhost/task/arn:aws:ecs:us-west-2:012:task/default/cafe",
		"http://localhost/task/arn:aws:ecs:us-west-2:012:task/cafe",
	}
	sadCases := []string{
		"http://localhost/task/arn:cafe",
		"http://localhost/task/arn:aws:ecs:us-west-2:012:task",
		"http://localhost/task/arn:aws:ecs:us-west-2:012:task/cafe/cafe/cafe",
	}
	for i, v := range happyCases {
		r, _ := http.NewRequest("GET", v, nil)
		if extractArn(r) == "" {
			t.Errorf("trial %d failed with url %s: ", i, v)
		}
	}
	for i, v := range sadCases {
		r, _ := http.NewRequest("GET", v, nil)
		if extractArn(r) != "" {
			t.Errorf("trial %d failed with url %s: ", i, v)
		}
	}
}
