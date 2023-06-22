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
	Address  string
	TenantId string
}

type AlertManager struct {
	address string

	client *httpClient
}

func New(config Config) (*AlertManager, error) {
	if config.Address == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.Address must not be empty", config)
	}

	client := httpClient{
		c:        http.Client{},
		tenantId: config.TenantId}

	return &AlertManager{
		address: config.Address,
		client:  &client,
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

	resp, err := am.client.Post(endpoint, "application/json", bytes.NewBuffer(jsonValues))
	if err != nil {
		return microerror.Mask(err)
	}

	if resp.StatusCode != 200 {
		return microerror.Maskf(executionFailedError, fmt.Sprintf("failed to create/update silence %#q, expected code 200, got %d", s.Comment, resp.StatusCode))
	}

	return nil
}

func (am *AlertManager) UpdateSilence(s *Silence) error {
	if s.ID == "" {
		return microerror.Maskf(executionFailedError, fmt.Sprintf("failed to update silence %#q, missing ID", s.Comment))
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

	return microerror.Maskf(notFoundError, fmt.Sprintf("failed to delete silence by comment %#q", comment))
}

func (am *AlertManager) ListSilences() ([]Silence, error) {
	endpoint := fmt.Sprintf("%s/api/v2/silences", am.address)

	var silences []Silence

	resp, err := am.client.Get(endpoint)
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

	resp, err := am.client.Delete(endpoint, "application/json")
	if err != nil {
		return microerror.Mask(err)
	}

	if resp.StatusCode != 200 {
		return microerror.Maskf(executionFailedError, fmt.Sprintf("failed to delete silence %#q, expected code 200, got %d", id, resp.StatusCode))
	}

	return nil
}
