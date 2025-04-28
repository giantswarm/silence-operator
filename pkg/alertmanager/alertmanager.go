package alertmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/giantswarm/microerror"
	"github.com/pkg/errors"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
)

const (
	CreatedBy                = "silence-operator"
	ValidUntilAnnotationName = "valid-until"
	DateOnlyLayout           = "2006-01-02"
)

// TODO Get rid of microerrors and use errors instead.
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
		return nil, microerror.Mask(err)
	}

	for _, s := range silences {
		if s.Comment == comment {
			return &s, nil
		}
	}

	return nil, microerror.Maskf(notFoundError, "failed to get silence with comment %#q", comment)
}

func (am *AlertManager) CreateSilence(s *Silence) error {
	endpoint := fmt.Sprintf("%s/api/v2/silences", am.address)

	jsonValues, err := json.Marshal(s)
	if err != nil {
		return microerror.Mask(err)
	}

	req, err := am.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(jsonValues))
	if err != nil {
		return microerror.Mask(err)
	}
	req.Header.Add("Content-Type", "application/json")

	if am.authentication {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", am.token))
	}

	resp, err := am.client.Do(req)
	if err != nil {
		return microerror.Mask(err)
	}
	defer resp.Body.Close() //nolint: errcheck

	if resp.StatusCode != 200 {
		return microerror.Maskf(executionFailedError, "failed to create/update silence %#q, expected code 200, got %d", s.Comment, resp.StatusCode)
	}

	return nil
}

func (am *AlertManager) UpdateSilence(s *Silence) error {
	if s.ID == "" {
		return microerror.Maskf(executionFailedError, "failed to update silence %#q, missing ID", s.Comment)
	}
	return am.CreateSilence(s)
}

func (am *AlertManager) DeleteSilenceByComment(comment string) error {
	silences, err := am.ListSilences()
	if err != nil {
		return microerror.Mask(err)
	}

	for _, s := range silences {
		if s.Comment == comment && s.CreatedBy == CreatedBy {
			return am.DeleteSilenceByID(s.ID)
		}
	}

	return microerror.Maskf(notFoundError, "failed to delete silence by comment %#q", comment)
}

func (am *AlertManager) ListSilences() ([]Silence, error) {
	endpoint := fmt.Sprintf("%s/api/v2/silences", am.address)

	var silences []Silence

	req, err := am.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if am.authentication {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", am.token))
	}

	resp, err := am.client.Do(req)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	defer resp.Body.Close() //nolint: errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	err = json.Unmarshal(body, &silences)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var filteredSilences []Silence
	{
		for _, silence := range silences {
			if silence.Status.State != "expired" {
				filteredSilences = append(filteredSilences, silence)
			}
		}
	}

	return filteredSilences, nil
}

func (am *AlertManager) DeleteSilenceByID(id string) error {
	endpoint := fmt.Sprintf("%s/api/v2/silence/%s", am.address, id)

	req, err := am.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return microerror.Mask(err)
	}

	req.Header.Add("Content-Type", "application/json")

	if am.authentication {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", am.token))
	}

	resp, err := am.client.Do(req)
	if err != nil {
		return microerror.Mask(err)
	}
	defer resp.Body.Close() //nolint: errcheck

	if resp.StatusCode != 200 {
		return microerror.Maskf(executionFailedError, "failed to delete silence %#q, expected code 200, got %d", id, resp.StatusCode)
	}

	return nil
}

func SilenceComment(silence *v1alpha1.Silence) string {
	return fmt.Sprintf("%s-%s", CreatedBy, silence.Name)
}

// SilenceEndsAt gets the expiry date for a given silence.
// The expiry date is retrieved from the annotation name configured by ValidUntilAnnotationName.
// The expected format is defined by DateOnlyLayout.
// It returns an invalidExpirationDateError in case the date format is invalid.
func SilenceEndsAt(silence *v1alpha1.Silence) (time.Time, error) {
	annotations := silence.GetAnnotations()

	// Check if the annotation exist otherwise return a date 100 years in the future.
	value, ok := annotations[ValidUntilAnnotationName]
	if !ok {
		return silence.GetCreationTimestamp().AddDate(100, 0, 0), nil
	}

	// Parse the date found in the annotation using RFC3339 (ISO8601) by default
	expirationDate, err := time.Parse(time.RFC3339, value)
	if err != nil {
		// If it fails, we try to parse it using date only (old way)
		expirationDate, err = time.Parse(DateOnlyLayout, value)
		if err != nil {
			return time.Time{}, microerror.Maskf(invalidExpirationDateError, "%s date %q does not match expected format %q: %v", ValidUntilAnnotationName, value, DateOnlyLayout, err)
		}
		// We shift the time to 8am UTC (9 CET or 10 CEST) to ensure silences do not expire at night.
		expirationDate = time.Date(expirationDate.Year(), expirationDate.Month(), expirationDate.Day(), 8, 0, 0, 0, time.UTC)
	}

	return expirationDate, nil
}
