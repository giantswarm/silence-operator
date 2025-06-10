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

// Client defines the interface for Alertmanager operations
type Client interface {
	GetSilenceByComment(comment string) (*Silence, error)
	CreateSilence(silence *Silence) error
	UpdateSilence(silence *Silence) error
	DeleteSilenceByComment(comment string) error
	DeleteSilenceByID(id string) error
}

type Config struct {
	Address        string
	Authentication bool
	BearerToken    string
	TenantId       string
}

type AlertManager struct {
	address        string
	authentication bool
	token          string
	tenantId       string
	client         *http.Client
}

func New(config Config) (*AlertManager, error) {
	if config.Address == "" {
		return nil, errors.Errorf("%T.Address must not be empty", config)
	}

	return &AlertManager{
		address:        config.Address,
		authentication: config.Authentication,
		token:          config.BearerToken,
		client:         http.DefaultClient,
		tenantId:       config.TenantId,
	}, nil
}

func (am *AlertManager) GetSilenceByComment(comment string) (*Silence, error) {
	silences, err := am.ListSilences()
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

func (am *AlertManager) CreateSilence(s *Silence) error {
	endpoint := fmt.Sprintf("%s%s", am.address, apiV2SilencesPath) // Use constant

	jsonValues, err := json.Marshal(s)
	if err != nil {
		return errors.WithStack(err)
	}

	req, err := am.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(jsonValues))
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

func (am *AlertManager) UpdateSilence(s *Silence) error {
	if s.ID == "" {
		return errors.Errorf("failed to update silence %#q, missing ID", s.Comment)
	}
	return am.CreateSilence(s)
}

func (am *AlertManager) DeleteSilenceByComment(comment string) error {
	silences, err := am.ListSilences()
	if err != nil {
		return errors.WithStack(err)
	}

	for _, s := range silences {
		if s.Comment == comment && s.CreatedBy == CreatedBy {
			return am.DeleteSilenceByID(s.ID)
		}
	}

	return errors.WithMessagef(ErrSilenceNotFound, "failed to delete silence by comment %#q", comment)
}

func (am *AlertManager) ListSilences() ([]Silence, error) {
	endpoint := fmt.Sprintf("%s%s", am.address, apiV2SilencesPath)

	var silences []Silence

	req, err := am.NewRequest(http.MethodGet, endpoint, nil)
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

func (am *AlertManager) DeleteSilenceByID(id string) error {
	// Use constant and url.PathEscape for safety if ID can contain special chars
	endpoint := fmt.Sprintf("%s%s/%s", am.address, apiV2SilencePath, url.PathEscape(id))

	req, err := am.NewRequest(http.MethodDelete, endpoint, nil)
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
