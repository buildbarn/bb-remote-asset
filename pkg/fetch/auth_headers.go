package fetch

import (
	"encoding/json"
	"net/http"
)

// AuthHeaders is a map from target URI to headers to be applied for the request
type AuthHeaders map[string]map[string]string

// NewAuthHeadersFromQualifier creates an AuthHeaders from the qualifier payload
func NewAuthHeadersFromQualifier(value string) (*AuthHeaders, error) {
	var ah AuthHeaders
	err := json.Unmarshal([]byte(value), &ah)
	return &ah, err
}

// ApplyHeaders mutates a http.Request to apply headers requested by the client.
func (ah AuthHeaders) ApplyHeaders(uri string, req *http.Request) {
	if headers, ok := ah[uri]; ok {
		for header, val := range headers {
			req.Header.Set(header, val)
		}
	}
}
