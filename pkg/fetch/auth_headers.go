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

// NewAuthHeaders creates an empty AuthHeaders
func NewAuthHeaders() *AuthHeaders {
	return &AuthHeaders{}
}

// AddHeader adds a header to the AuthHeaders
func (ah AuthHeaders) AddHeader(uri, header, value string) {
	if _, ok := ah[uri]; !ok {
		ah[uri] = make(map[string]string)
	}
	ah[uri][header] = value
}

// ApplyHeaders mutates a http.Request to apply headers requested by the client.
func (ah AuthHeaders) ApplyHeaders(uri string, req *http.Request) {
	if headers, ok := ah[uri]; ok {
		for header, val := range headers {
			req.Header.Set(header, val)
		}
	}
}
