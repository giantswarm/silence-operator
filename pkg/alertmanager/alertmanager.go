package alertmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/silence-operator/pkg/config"
)

const (
	CreatedBy                = "silence-operator"
	ValidUntilAnnotationName = "valid-until"
	DateOnlyLayout           = "2006-01-02"
	// Define API paths as constants
	apiV2SilencesPath = "/api/v2/silences"
	apiV2SilencePath  = "/api/v2/silence"
	// Define state constant
	SilenceStateExpired = "expired"
)

var (
	ErrSilenceNotFound = errors.New("silence not found")
)

// Client defines the contract for alertmanager operations
type Client interface {
	GetSilenceByComment(comment string, tenant string) (*Silence, error)
	CreateSilence(s *Silence, tenant string) error
	UpdateSilence(s *Silence, tenant string) error
	DeleteSilenceByComment(comment string, tenant string) error
	DeleteSilenceByID(id string, tenant string) error
	ListSilences(tenant string) ([]Silence, error)
}

// Ensure Alertmanager implements Client
var _ Client = (*Alertmanager)(nil)

type Alertmanager struct {
	address        string
	authentication bool
	token          string
	tenantId       string
	client         *http.Client
}

func New(config config.Config) (*Alertmanager, error) {
	if config.Address == "" {
		return nil, errors.Errorf("%T.Address must not be empty", config)
	}

	return &Alertmanager{
		address:        config.Address,
		authentication: config.Authentication,
		token:          config.BearerToken,
		client:         http.DefaultClient,
		tenantId:       config.TenantId,
	}, nil
}

func (am *Alertmanager) GetSilenceByComment(comment string, tenant string) (*Silence, error) {
	silences, err := am.ListSilences(tenant)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, s := range silences {
		if s.Comment == comment {
			return &s, nil
		}
	}

	return nil, errors.WithMessagef(ErrSilenceNotFound, "failed to get silence with comment %#q", comment)
}

func (am *Alertmanager) CreateSilence(s *Silence, tenant string) error {
	endpoint := fmt.Sprintf("%s%s", am.address, apiV2SilencesPath)

	jsonValues, err := json.Marshal(s)
	if err != nil {
		return errors.WithStack(err)
	}

	req, err := am.newRequestWithTenant(http.MethodPost, endpoint, bytes.NewBuffer(jsonValues), tenant)
	if err != nil {
		return errors.WithStack(err)
	}
	req.Header.Add("Content-Type", "application/json")

	if am.authentication {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", am.token))
	}

	resp, err := am.client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close() //nolint: errcheck

	if resp.StatusCode != 200 {
		return errors.Errorf("failed to create/update silence %#q, expected code 200, got %d", s.Comment, resp.StatusCode)
	}

	return nil
}

func (am *Alertmanager) UpdateSilence(s *Silence, tenant string) error {
	if s.ID == "" {
		return errors.Errorf("failed to update silence %#q, missing ID", s.Comment)
	}
	return am.CreateSilence(s, tenant)
}

func (am *Alertmanager) DeleteSilenceByComment(comment string, tenant string) error {
	silences, err := am.ListSilences(tenant)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, s := range silences {
		if s.Comment == comment && s.CreatedBy == CreatedBy {
			return am.DeleteSilenceByID(s.ID, tenant)
		}
	}

	return errors.WithMessagef(ErrSilenceNotFound, "failed to delete silence by comment %#q", comment)
}

func (am *Alertmanager) DeleteSilenceByID(id string, tenant string) error {
	endpoint := fmt.Sprintf("%s%s/%s", am.address, apiV2SilencePath, url.PathEscape(id))

	req, err := am.newRequestWithTenant(http.MethodDelete, endpoint, nil, tenant)
	if err != nil {
		return errors.WithStack(err)
	}

	req.Header.Add("Content-Type", "application/json")

	if am.authentication {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", am.token))
	}

	resp, err := am.client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close() //nolint: errcheck

	if resp.StatusCode != 200 {
		return errors.WithMessagef(errors.WithStack(err), "failed to delete silence %#q, expected code 200, got %d", id, resp.StatusCode)
	}

	return nil
}

func (am *Alertmanager) ListSilences(tenant string) ([]Silence, error) {
	endpoint := fmt.Sprintf("%s%s", am.address, apiV2SilencesPath)

	var silences []Silence

	req, err := am.newRequestWithTenant(http.MethodGet, endpoint, nil, tenant)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if am.authentication {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", am.token))
	}

	resp, err := am.client.Do(req)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer resp.Body.Close() //nolint: errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = json.Unmarshal(body, &silences)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var filteredSilences []Silence
	for _, silence := range silences {
		if silence.Status != nil && silence.Status.State != SilenceStateExpired {
			filteredSilences = append(filteredSilences, silence)
		}
	}

	return filteredSilences, nil
}

// newRequestWithTenant creates an HTTP request with tenant-specific headers
func (am *Alertmanager) newRequestWithTenant(method, url string, body io.Reader, tenant string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	// Determine which tenant to use: parameter takes precedence over instance field
	effectiveTenant := tenant
	if effectiveTenant == "" {
		effectiveTenant = am.tenantId
	}

	// Add tenant header if tenant is specified
	if effectiveTenant != "" {
		req.Header.Add("X-Scope-OrgID", effectiveTenant)
	}

	return req, nil
}

func SilenceComment(silence client.Object) string {
	if silence.GetNamespace() != "" {
		return fmt.Sprintf("%s-%s-%s", CreatedBy, silence.GetNamespace(), silence.GetName())
	}
	return fmt.Sprintf("%s-%s", CreatedBy, silence.GetName())
}

// SilenceEndsAt gets the expiry date for a given silence.
// The expiry date is retrieved from the annotation name configured by ValidUntilAnnotationName.
// The expected format is defined by DateOnlyLayout.
// It returns an invalidExpirationDateError in case the date format is invalid.
func SilenceEndsAt(silence client.Object) (time.Time, error) {
	annotations := silence.GetAnnotations()

	// Check if the annotation exist otherwise return a date 100 years in the future.
	value, ok := annotations[ValidUntilAnnotationName]
	if !ok {
		return silence.GetCreationTimestamp().AddDate(100, 0, 0), nil
	}

	expirationDate, errRFC3339 := time.Parse(time.RFC3339, value)
	if errRFC3339 == nil {
		// Parsed successfully with RFC3339
		return expirationDate, nil
	}

	// If RFC3339 parsing fails, try parsing using the legacy DateOnlyLayout
	expirationDate, errDateOnly := time.Parse(DateOnlyLayout, value)
	if errDateOnly != nil {
		// Combine both errors in the message. Wrap errRFC3339 to preserve stack trace.
		return time.Time{}, errors.Wrapf(errRFC3339, "annotation %q date %q does not match expected formats %q or %q (legacy format error: %v)", ValidUntilAnnotationName, value, time.RFC3339, DateOnlyLayout, errDateOnly)
	}
	// We shift the time to 8am UTC (9 CET or 10 CEST) to ensure silences do not expire at night.
	// TODO move this rule to a config?
	expirationDate = time.Date(expirationDate.Year(), expirationDate.Month(), expirationDate.Day(), 8, 0, 0, 0, time.UTC)
	return expirationDate, nil
}
