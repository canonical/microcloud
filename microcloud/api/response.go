package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/lxc/lxd/lxd/response"
	"github.com/lxc/lxd/shared/api"
)

// Response represents a response returned from a MicroCloud service.
type Response struct {
	response *http.Response
}

// NewResponse wraps the given http.Response as a Response.
func NewResponse(response *http.Response) response.Response {
	return &Response{response: response}
}

// Render implements response.Response for Response, enabling use with a rest.EndpointAction Handler function.
func (r *Response) Render(w http.ResponseWriter) error {
	decoder := json.NewDecoder(r.response.Body)

	var responseRaw *api.ResponseRaw
	err := decoder.Decode(&responseRaw)
	if err != nil {
		return err
	}

	return r.render(responseRaw, w)
}

// String implements response.Response for the Response.
func (r *Response) String() string {
	return fmt.Sprintf("%s - %s", r.response.Proto, r.response.Status)
}

// render copies the response status code and headers, and writes the transformed response body to the http.ResponseWriter.
func (r *Response) render(responseRaw *api.ResponseRaw, w http.ResponseWriter) error {
	responseBody, err := json.Marshal(responseRaw)
	if err != nil {
		return err
	}

	for key, value := range r.response.Header {
		if key == "Content-Length" {
			w.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
			continue
		}

		for _, v := range value {
			w.Header().Set(key, v)
		}
	}

	w.WriteHeader(r.response.StatusCode)

	_, err = w.Write(responseBody)
	return err
}
