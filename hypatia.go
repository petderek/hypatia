package hypatia

import "net/http"

type TPEndpoint struct {
	tpc       TaskProtectionClient
	Writeable bool
}

func (tp *TPEndpoint) ServeHTTP(res http.ResponseWriter, req *http.Request) {

}

type LocalHealthEndpoint struct {
}
