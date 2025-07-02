package alertmanager

import (
	"io"
	"net/http"
)

// NewRequest creates a new http.Request with the given method, url and body.
// It adds the tenantId as X-Scope-OrgID header to the request if it is set.
// Deprecated: Use newRequestWithTenant for better tenant control.
func (am *Alertmanager) NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	return am.newRequestWithTenant(method, url, body, "")
}
