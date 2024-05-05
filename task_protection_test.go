package hypatia

import (
	"encoding/json"
	"testing"
)

const exampleGoodResponse = `{"protection":{"ExpirationDate":null,"ProtectionEnabled":false,"TaskArn":"arn:aws:ecs:us-west-2:0123456789:task/forgettingSarahUnmarshal"}}`

func TestMarshal(t *testing.T) {
	var res TaskProtectionResponse
	err := json.Unmarshal([]byte(exampleGoodResponse), &res)
	if err != nil {
		t.Fatal("error unmarshal: ", err)
	}
	t.Log(res)
}
