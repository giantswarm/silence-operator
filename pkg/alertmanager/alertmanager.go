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
	SilenceEndsAt(silence client.Object) (time.Time, error)
}

// Ensure Alertmanager implements Client
var _ Client = (*Alertmanager)(nil)

type Alertmanager struct {
	address           string
	authentication    bool
	token             string
	tenantId          string
	client            *http.Client
	expirationHour    int
	expirationMinute  int
	expirationLoc     *time.Location
}

func New(config config.Config) (*Alertmanager, error) {
	if config.Address == "" {
		return nil, errors.Errorf("%T.Address must not be empty", config)
	}

	expirationTime := config.ExpirationTime
	if expirationTime == "" {
		expirationTime = "08:00Z"
	}

	parsed, err := time.Parse("15:04Z07:00", expirationTime)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid expiration-time %q: must be ISO 8601 HH:MM±HH:MM (e.g. \"08:00Z\", \"08:00+01:00\")", expirationTime)
	}
	_, offsetSeconds := parsed.Zone()
	var loc *time.Location
	if offsetSeconds == 0 {
		loc = time.UTC
	} else {
		loc = time.FixedZone(fmt.Sprintf("%+03d:%02d", offsetSeconds/3600, (offsetSeconds%3600)/60), offsetSeconds)
	}

	return &Alertmanager{
		address:          config.Address,
		authentication:   config.Authentication,
		token:            config.BearerToken,
		client:           http.DefaultClient,
		tenantId:         config.TenantId,
		expirationHour:   parsed.Hour(),
		expirationMinute: parsed.Minute(),
		expirationLoc:    loc,
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

	req, err := am.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(jsonValues), tenant)
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

func (am *Alertmanager) ListSilences(tenant string) ([]Silence, error) {
	endpoint := fmt.Sprintf("%s%s", am.address, apiV2SilencesPath)

	var silences []Silence

	req, err := am.NewRequest(http.MethodGet, endpoint, nil, tenant)
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

func (am *Alertmanager) DeleteSilenceByID(id string, tenant string) error {
	endpoint := fmt.Sprintf("%s%s/%s", am.address, apiV2SilencePath, url.PathEscape(id))

	req, err := am.NewRequest(http.MethodDelete, endpoint, nil, tenant)
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
func (am *Alertmanager) SilenceEndsAt(silence client.Object) (time.Time, error) {
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
	// Shift to the configured expiration time to ensure silences do not expire at night.
	expirationDate = time.Date(expirationDate.Year(), expirationDate.Month(), expirationDate.Day(), am.expirationHour, am.expirationMinute, 0, 0, am.expirationLoc)
	return expirationDate, nil
}
