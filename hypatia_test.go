package hypatia

import (
	"net/http"
	"testing"
)

func TestServe(t *testing.T) {
	http.ListenAndServe("", nil)
}
