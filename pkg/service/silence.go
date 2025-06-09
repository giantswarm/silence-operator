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

// Package service contains the business logic layer for silence management.
package service

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/silence-operator/pkg/alertmanager"
)

type SilenceService struct {
	alertmanager alertmanager.Client
	clock        clock.Clock
}

func NewSilenceService(am alertmanager.Client, clock clock.Clock) *SilenceService {
	if am == nil {
		panic("alertmanager client cannot be nil")
	}
	if clock == nil {
		panic("clock cannot be nil")
	}
	return &SilenceService{
		alertmanager: am,
		clock:        clock,
	}
}

func (s *SilenceService) SyncSilence(ctx context.Context, newSilence *alertmanager.Silence) (bool, error) {
	if s == nil {
		return false, errors.New("service is nil")
	}
	if s.alertmanager == nil {
		return false, errors.New("alertmanager client is nil")
	}
	if s.clock == nil {
		return false, errors.New("clock is nil")
	}

	logger := log.FromContext(ctx).WithName("SilenceService")

	if newSilence == nil {
		logger.Error(nil, "Validation failed: silence cannot be nil")
		return false, errors.New("silence cannot be nil")
	}

	logger.Info("Starting silence synchronization", "comment", newSilence.Comment)

	if newSilence.Comment == "" {
		logger.Error(nil, "Validation failed: silence comment cannot be empty")
		return false, errors.New("silence comment cannot be empty")
	}
	if newSilence.EndsAt.IsZero() {
		logger.Error(nil, "Validation failed: silence end time must be specified", "comment", newSilence.Comment)
		return false, errors.New("silence end time must be specified")
	}

	now := s.clock.Now()
	logger.V(1).Info("Validated silence input",
		"comment", newSilence.Comment,
		"endsAt", newSilence.EndsAt,
		"startsAt", newSilence.StartsAt,
		"currentTime", now,
		"matcherCount", len(newSilence.Matchers))

	logger.V(1).Info("Checking if silence exists in Alertmanager", "comment", newSilence.Comment)
	existingSilence, err := s.alertmanager.GetSilenceByComment(newSilence.Comment)
	if err != nil && !errors.Is(err, alertmanager.ErrSilenceNotFound) {
		logger.Error(err, "Failed to retrieve silence from Alertmanager", "comment", newSilence.Comment)
		return false, errors.WithStack(err)
	}

	if errors.Is(err, alertmanager.ErrSilenceNotFound) {
		logger.Info("Silence not found in Alertmanager", "comment", newSilence.Comment)

		if newSilence.EndsAt.After(now) {
			logger.Info("Creating new silence in Alertmanager",
				"comment", newSilence.Comment,
				"endsAt", newSilence.EndsAt,
				"duration", newSilence.EndsAt.Sub(now))

			err = s.alertmanager.CreateSilence(newSilence)
			if err != nil {
				logger.Error(err, "Failed to create silence in Alertmanager", "comment", newSilence.Comment)
				return false, errors.WithStack(err)
			}

			logger.Info("Successfully created new silence", "comment", newSilence.Comment)
			return true, nil
		} else {
			logger.Info("Silence already expired, skipping creation",
				"comment", newSilence.Comment,
				"endsAt", newSilence.EndsAt,
				"expiredSince", now.Sub(newSilence.EndsAt))
			return false, nil
		}
	}

	logger.Info("Found existing silence in Alertmanager",
		"comment", newSilence.Comment,
		"id", existingSilence.ID,
		"currentEndsAt", existingSilence.EndsAt,
		"newEndsAt", newSilence.EndsAt)

	if newSilence.EndsAt.Before(now) {
		logger.Info("Deleting expired silence from Alertmanager",
			"comment", newSilence.Comment,
			"id", existingSilence.ID,
			"endsAt", newSilence.EndsAt,
			"expiredSince", now.Sub(newSilence.EndsAt))

		err = s.alertmanager.DeleteSilenceByID(existingSilence.ID)
		if err != nil {
			logger.Error(err, "Failed to delete expired silence",
				"comment", newSilence.Comment,
				"id", existingSilence.ID)
			return false, errors.WithStack(err)
		}

		logger.Info("Successfully deleted expired silence", "comment", newSilence.Comment, "id", existingSilence.ID)
		return true, nil
	}

	logger.V(1).Info("Checked if silence update is needed",
		"comment", newSilence.Comment)

	if s.updateNeeded(existingSilence, newSilence) {
		logger.Info("Updating existing silence in Alertmanager",
			"comment", newSilence.Comment,
			"id", existingSilence.ID,
			"oldEndsAt", existingSilence.EndsAt,
			"newEndsAt", newSilence.EndsAt)

		newSilence.ID = existingSilence.ID
		err = s.alertmanager.UpdateSilence(newSilence)
		if err != nil {
			logger.Error(err, "Failed to update silence in Alertmanager",
				"comment", newSilence.Comment,
				"id", existingSilence.ID)
			return false, errors.WithStack(err)
		}

		logger.Info("Successfully updated silence", "comment", newSilence.Comment, "id", existingSilence.ID)
		return true, nil
	}

	logger.Info("No changes needed for silence", "comment", newSilence.Comment, "id", existingSilence.ID)
	return false, nil
}

func (s *SilenceService) DeleteSilence(ctx context.Context, silence *alertmanager.Silence) error {
	if s == nil {
		return errors.New("service is nil")
	}
	if s.alertmanager == nil {
		return errors.New("alertmanager client is nil")
	}
	if silence == nil {
		return errors.New("silence cannot be nil")
	}

	logger := log.FromContext(ctx).WithName("SilenceService")

	logger.Info("Deleting silence from Alertmanager", "comment", silence.Comment)

	err := s.alertmanager.DeleteSilenceByComment(silence.Comment)
	if err != nil {
		if errors.Is(err, alertmanager.ErrSilenceNotFound) {
			logger.Info("Silence already deleted from Alertmanager", "comment", silence.Comment)
			return nil
		}
		logger.Error(err, "Failed to delete silence from Alertmanager", "comment", silence.Comment)
		return errors.Wrap(err, "failed to delete silence from Alertmanager")
	}

	logger.Info("Successfully deleted silence from Alertmanager", "comment", silence.Comment)
	return nil
}

// updateNeeded returns true when a silence needs to be updated.
// It performs a deep comparison of all relevant fields.
func (s *SilenceService) updateNeeded(existingSilence, newSilence *alertmanager.Silence) bool {
	return !reflect.DeepEqual(existingSilence.Matchers, newSilence.Matchers) ||
		!existingSilence.EndsAt.Equal(newSilence.EndsAt)
}
