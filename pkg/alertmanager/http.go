package alertmanager

import (
	"io"
	"net/http"
)

// NewRequest creates a new http.Request with the given method, url and body.
// It adds the tenantId as X-Scope-OrgID header to the request if it is set.
// The tenant parameter takes precedence over the instance tenantId.
func (am *Alertmanager) NewRequest(method, url string, body io.Reader, tenant string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	// Determine which tenant to use: parameter takes precedence over instance field
	effectiveTenant := am.tenantId
	if tenant != "" {
		effectiveTenant = tenant
	}

	// Add tenant header if tenant is specified
	if effectiveTenant != "" {
		req.Header.Add("X-Scope-OrgID", effectiveTenant)
	}

	return req, nil
}
