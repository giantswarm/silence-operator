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
	"reflect"
	"time"

	"github.com/pkg/errors"

	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

// SilenceService provides business logic for managing silences
type SilenceService struct {
	alertmanager alertmanager.Client
}

// NewSilenceService creates a new silence service
func NewSilenceService(alertmanager alertmanager.Client) *SilenceService {
	return &SilenceService{
		alertmanager: alertmanager,
	}
}

// SyncSilence handles the creation or update of a silence
func (s *SilenceService) SyncSilence(ctx context.Context, newSilence *alertmanager.Silence) error {
	now := time.Now()

	// Get existing silence by comment (use empty tenant for backward compatibility)
	existingSilence, err := s.alertmanager.GetSilenceByComment(newSilence.Comment, "")
	if err != nil && !errors.Is(err, alertmanager.ErrSilenceNotFound) {
		return errors.Wrap(err, "failed to get silence from Alertmanager")
	}

	if errors.Is(err, alertmanager.ErrSilenceNotFound) {
		if newSilence.EndsAt.After(now) {
			err := s.alertmanager.CreateSilence(newSilence, "")
			if err != nil {
				return errors.Wrap(err, "failed to create silence in Alertmanager")
			}
		}
		return nil
	}

	if newSilence.EndsAt.Before(now) {
		err := s.alertmanager.DeleteSilenceByID(existingSilence.ID, "")
		if err != nil {
			return errors.Wrap(err, "failed to delete expired silence from Alertmanager")
		}
		return nil
	}

	if s.updateNeeded(existingSilence, newSilence) {
		newSilence.ID = existingSilence.ID
		err := s.alertmanager.UpdateSilence(newSilence, "")
		if err != nil {
			return errors.Wrap(err, "failed to update silence in Alertmanager")
		}
		return nil
	}

	// No changes needed
	return nil
}

// DeleteSilence handles the deletion of a silence
func (s *SilenceService) DeleteSilence(ctx context.Context, comment string) error {
	err := s.alertmanager.DeleteSilenceByComment(comment, "")
	if err != nil {
		// If the silence is already gone in Alertmanager, treat it as success
		if errors.Is(err, alertmanager.ErrSilenceNotFound) {
			return nil
		}
		// For other errors, return the error to retry
		return errors.Wrap(err, "failed to delete silence from Alertmanager")
	}

	return nil
}

// updateNeeded returns true when silence needs to be updated
func (s *SilenceService) updateNeeded(existingSilence, newSilence *alertmanager.Silence) bool {
	return !reflect.DeepEqual(existingSilence.Matchers, newSilence.Matchers) ||
		!existingSilence.EndsAt.Equal(newSilence.EndsAt)
}
