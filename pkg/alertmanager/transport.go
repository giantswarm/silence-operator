package alertmanager

import "net/http"

type customTransport struct {
	tenantId string
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.tenantId != "" {
		req.Header.Add("X-Scope-OrgID", t.tenantId)
	}
	return http.DefaultTransport.RoundTrip(req)
}
