package alertmanager

import (
	"io"
	"net/http"
)

// NewRequest creates a new http.Request with the given method, url and body.
// It adds the tenantId as X-Scope-OrgID header to the request if it is set.
func (am *AlertManager) NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if am.tenantId != "" {
		req.Header.Add("X-Scope-OrgID", am.tenantId)
	}

	return req, nil
}
