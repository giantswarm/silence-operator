package alertmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/silence-operator/service/controller/key"
)

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
		return nil, microerror.Maskf(invalidConfigError, "%T.Address must not be empty", config)
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
	if resp.StatusCode != 200 {
		return microerror.Maskf(executionFailedError, "failed to create/update silence %#q, expected code 200, got %d", s.Comment, resp.StatusCode)
	}
	defer resp.Body.Close()

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
		if s.Comment == comment && s.CreatedBy == key.CreatedBy {
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

	defer resp.Body.Close()
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

	if resp.StatusCode != 200 {
		return microerror.Maskf(executionFailedError, "failed to delete silence %#q, expected code 200, got %d", id, resp.StatusCode)
	}

	return nil
}
