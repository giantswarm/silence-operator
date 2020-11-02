package alertmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/silence-operator/service/controller/key"
)

type Config struct {
	Address string
}

type AlertManager struct {
	address string

	httpClient *http.Client
}

func New(config Config) (*AlertManager, error) {
	if config.Address == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.Address must not be empty", config)
	}

	httpClient := &http.Client{}

	return &AlertManager{
		address:    config.Address,
		httpClient: httpClient,
	}, nil
}

func (am *AlertManager) GetSilence(opts *GetOptions) (*Silence, error) {
	silences, err := am.ListSilences()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	for _, s := range silences {
		if s.Comment == opts.Comment {
			return &s, nil
		}
	}

	return nil, microerror.Maskf(notFoundError, opts.Comment)
}

func (am *AlertManager) CreateSilence(s *Silence) error {
	endpoint := fmt.Sprintf("%s/api/v2/silences", am.address)

	jsonValues, err := json.Marshal(s)
	if err != nil {
		return microerror.Mask(err)
	}

	resp, err := am.httpClient.Post(endpoint, "application/json", bytes.NewBuffer(jsonValues))
	if err != nil {
		return microerror.Mask(err)
	}

	if resp.StatusCode != 200 {
		return microerror.Maskf(executionFailedError, fmt.Sprintf("failed to create silence %#q, expected code 200, got %d", s.Comment, resp.StatusCode))
	}

	return nil
}

func (am *AlertManager) DeleteSilence(id string, opts *DeleteOptions) error {

	silenceID := id
	if opts != nil {
		silences, err := am.ListSilences()
		if err != nil {
			return microerror.Mask(err)
		}

		for _, s := range silences {
			if s.Comment == opts.Comment && s.CreatedBy == key.CreatedBy {
				silenceID = s.ID
				break
			}
		}
	}

	if silenceID == "" {
		return nil
	}

	endpoint := fmt.Sprintf("%s/api/v2/silence/%s", am.address, silenceID)

	req, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		return microerror.Mask(err)
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		return microerror.Mask(err)
	}

	if resp.StatusCode != 200 {
		return microerror.Maskf(executionFailedError, fmt.Sprintf("failed to create silence %#q, expected code 200, got %d", opts.Comment, resp.StatusCode))
	}

	return nil
}

func (am *AlertManager) ListSilences() ([]Silence, error) {
	endpoint := fmt.Sprintf("%s/api/v2/silences", am.address)

	var silences []Silence

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	resp, err := am.httpClient.Do(req)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
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
