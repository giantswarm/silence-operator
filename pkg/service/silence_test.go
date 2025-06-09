/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/clock"
	testingclock "k8s.io/utils/clock/testing"

	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

// MockAlertmanagerClient implements the alertmanager.Client interface for testing
type MockAlertmanagerClient struct {
	silences map[string]*alertmanager.Silence
}

func NewMockAlertmanagerClient() *MockAlertmanagerClient {
	return &MockAlertmanagerClient{
		silences: make(map[string]*alertmanager.Silence),
	}
}

func (m *MockAlertmanagerClient) GetSilenceByComment(comment string) (*alertmanager.Silence, error) {
	for _, silence := range m.silences {
		if silence.Comment == comment {
			return silence, nil
		}
	}
	return nil, alertmanager.ErrSilenceNotFound
}

func (m *MockAlertmanagerClient) CreateSilence(silence *alertmanager.Silence) error {
	silence.ID = "test-id-" + silence.Comment
	m.silences[silence.ID] = silence
	return nil
}

func (m *MockAlertmanagerClient) UpdateSilence(silence *alertmanager.Silence) error {
	if _, exists := m.silences[silence.ID]; !exists {
		return alertmanager.ErrSilenceNotFound
	}
	m.silences[silence.ID] = silence
	return nil
}

func (m *MockAlertmanagerClient) DeleteSilenceByComment(comment string) error {
	for id, silence := range m.silences {
		if silence.Comment == comment {
			delete(m.silences, id)
			return nil
		}
	}
	return alertmanager.ErrSilenceNotFound
}

func (m *MockAlertmanagerClient) DeleteSilenceByID(id string) error {
	if _, exists := m.silences[id]; !exists {
		return alertmanager.ErrSilenceNotFound
	}
	delete(m.silences, id)
	return nil
}

func (m *MockAlertmanagerClient) ListSilences() ([]alertmanager.Silence, error) {
	var silences []alertmanager.Silence
	for _, silence := range m.silences {
		silences = append(silences, *silence)
	}
	return silences, nil
}

func TestNewSilenceService_PanicPrevention(t *testing.T) {
	t.Run("panic when alertmanager client is nil", func(t *testing.T) {
		assert.Panics(t, func() {
			NewSilenceService(nil, clock.RealClock{})
		})
	})

	t.Run("panic when clock is nil", func(t *testing.T) {
		mockClient := NewMockAlertmanagerClient()
		assert.Panics(t, func() {
			NewSilenceService(mockClient, nil)
		})
	})

	t.Run("successful creation with valid parameters", func(t *testing.T) {
		mockClient := NewMockAlertmanagerClient()
		assert.NotPanics(t, func() {
			service := NewSilenceService(mockClient, clock.RealClock{})
			assert.NotNil(t, service)
		})
	})
}

func TestSilenceService_SyncSilence_CreateNew(t *testing.T) {
	mockClient := NewMockAlertmanagerClient()
	fixedTime := time.Date(2025, 6, 7, 12, 0, 0, 0, time.UTC)
	fakeClock := testingclock.NewFakeClock(fixedTime)

	service := NewSilenceService(mockClient, fakeClock)

	silence := &alertmanager.Silence{
		Comment:   "test-silence",
		CreatedBy: "test-user",
		EndsAt:    fixedTime.Add(time.Hour),
		Matchers: []alertmanager.Matcher{
			{Name: "alertname", Value: "test", IsEqual: true, IsRegex: false},
		},
	}

	result, err := service.SyncSilence(context.Background(), silence)
	require.NoError(t, err)
	assert.True(t, result) // Should return true since silence was created

	// Verify the silence was created
	createdSilence, err := mockClient.GetSilenceByComment("test-silence")
	require.NoError(t, err)
	assert.Equal(t, "test-silence", createdSilence.Comment)
}

func TestSilenceService_SyncSilence_SkipExpired(t *testing.T) {
	mockClient := NewMockAlertmanagerClient()
	fixedTime := time.Date(2025, 6, 7, 12, 0, 0, 0, time.UTC)
	fakeClock := testingclock.NewFakeClock(fixedTime)

	service := NewSilenceService(mockClient, fakeClock)

	silence := &alertmanager.Silence{
		Comment:   "expired-silence",
		CreatedBy: "test-user",
		EndsAt:    fixedTime.Add(-time.Hour), // Already expired
		Matchers: []alertmanager.Matcher{
			{Name: "alertname", Value: "test", IsEqual: true, IsRegex: false},
		},
	}

	result, err := service.SyncSilence(context.Background(), silence)
	require.NoError(t, err)
	assert.False(t, result) // Should return false since expired silence was skipped

	// Verify no silence was created
	_, err = mockClient.GetSilenceByComment("expired-silence")
	assert.Equal(t, alertmanager.ErrSilenceNotFound, err)
}

func TestSilenceService_SyncSilence_UpdateExisting(t *testing.T) {
	mockClient := NewMockAlertmanagerClient()
	fixedTime := time.Date(2025, 6, 7, 12, 0, 0, 0, time.UTC)
	fakeClock := testingclock.NewFakeClock(fixedTime)

	service := NewSilenceService(mockClient, fakeClock)

	// Create an existing silence
	existingSilence := &alertmanager.Silence{
		ID:        "test-id",
		Comment:   "test-silence",
		CreatedBy: "test-user",
		EndsAt:    fixedTime.Add(time.Hour),
		Matchers: []alertmanager.Matcher{
			{Name: "alertname", Value: "test", IsEqual: true, IsRegex: false},
		},
	}
	mockClient.silences["test-id"] = existingSilence

	// Update with different matcher value
	newSilence := &alertmanager.Silence{
		Comment:   "test-silence",
		CreatedBy: "test-user",
		EndsAt:    fixedTime.Add(time.Hour),
		Matchers: []alertmanager.Matcher{
			{Name: "alertname", Value: "new-value", IsEqual: true, IsRegex: false},
		},
	}

	result, err := service.SyncSilence(context.Background(), newSilence)
	require.NoError(t, err)
	assert.True(t, result) // Should return true since silence was updated

	// Verify the silence was updated
	retrievedSilence, err := mockClient.GetSilenceByComment("test-silence")
	require.NoError(t, err)
	assert.Equal(t, "new-value", retrievedSilence.Matchers[0].Value)
}

func TestSilenceService_SyncSilence_UpdateNeeded(t *testing.T) {
	fixedTime := time.Date(2025, 6, 7, 12, 0, 0, 0, time.UTC)
	fakeClock := testingclock.NewFakeClock(fixedTime)

	tests := []struct {
		name         string
		newSilence   *alertmanager.Silence
		expectUpdate bool
	}{
		{
			name: "no change needed",
			newSilence: &alertmanager.Silence{
				Comment:   "test-silence",
				CreatedBy: "test-user",
				EndsAt:    fixedTime.Add(time.Hour),
				Matchers: []alertmanager.Matcher{
					{Name: "alertname", Value: "test", IsEqual: true, IsRegex: false},
				},
			},
			expectUpdate: false,
		},
		{
			name: "different end time",
			newSilence: &alertmanager.Silence{
				Comment:   "test-silence",
				CreatedBy: "test-user",
				EndsAt:    fixedTime.Add(2 * time.Hour), // Different end time
				Matchers: []alertmanager.Matcher{
					{Name: "alertname", Value: "test", IsEqual: true, IsRegex: false},
				},
			},
			expectUpdate: true,
		},
		{
			name: "different matcher value",
			newSilence: &alertmanager.Silence{
				Comment:   "test-silence",
				CreatedBy: "test-user",
				EndsAt:    fixedTime.Add(time.Hour),
				Matchers: []alertmanager.Matcher{
					{Name: "alertname", Value: "different", IsEqual: true, IsRegex: false},
				},
			},
			expectUpdate: true,
		},
		{
			name: "different number of matchers",
			newSilence: &alertmanager.Silence{
				Comment:   "test-silence",
				CreatedBy: "test-user",
				EndsAt:    fixedTime.Add(time.Hour),
				Matchers: []alertmanager.Matcher{
					{Name: "alertname", Value: "test", IsEqual: true, IsRegex: false},
					{Name: "instance", Value: "localhost", IsEqual: true, IsRegex: false},
				},
			},
			expectUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh mock client for each test
			mockClient := NewMockAlertmanagerClient()
			service := NewSilenceService(mockClient, fakeClock)

			// Set up existing silence in mock client
			existingSilence := &alertmanager.Silence{
				ID:        "test-id",
				Comment:   "test-silence",
				CreatedBy: "test-user",
				EndsAt:    fixedTime.Add(time.Hour),
				Matchers: []alertmanager.Matcher{
					{Name: "alertname", Value: "test", IsEqual: true, IsRegex: false},
				},
			}
			mockClient.silences["test-id"] = existingSilence

			result, err := service.SyncSilence(context.Background(), tt.newSilence)
			require.NoError(t, err)
			assert.Equal(t, tt.expectUpdate, result)
		})
	}
}

func TestSilenceService_SyncSilence_NilService(t *testing.T) {
	var service *SilenceService

	result, err := service.SyncSilence(context.Background(), &alertmanager.Silence{
		Comment: "test",
		EndsAt:  time.Now().Add(time.Hour),
	})

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service is nil")
}

func TestSilenceService_DeleteSilence(t *testing.T) {
	mockClient := NewMockAlertmanagerClient()
	service := NewSilenceService(mockClient, clock.RealClock{})

	// Create a silence to delete
	silence := &alertmanager.Silence{
		ID:      "test-id",
		Comment: "test-silence",
	}
	mockClient.silences["test-id"] = silence

	err := service.DeleteSilence(context.Background(), silence)
	require.NoError(t, err)

	// Verify the silence was deleted
	_, err = mockClient.GetSilenceByComment("test-silence")
	assert.Equal(t, alertmanager.ErrSilenceNotFound, err)
}

func TestSilenceService_DeleteSilence_AlreadyDeleted(t *testing.T) {
	mockClient := NewMockAlertmanagerClient()
	service := NewSilenceService(mockClient, clock.RealClock{})

	silence := &alertmanager.Silence{
		Comment: "non-existent-silence",
	}

	// Should not return an error if silence is already deleted
	err := service.DeleteSilence(context.Background(), silence)
	assert.NoError(t, err)
}

func TestSilenceService_DeleteSilence_NilService(t *testing.T) {
	var service *SilenceService

	err := service.DeleteSilence(context.Background(), &alertmanager.Silence{
		Comment: "test",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service is nil")
}
