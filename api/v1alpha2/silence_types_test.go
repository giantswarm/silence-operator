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

package v1alpha2_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/giantswarm/silence-operator/api/v1alpha2"
)

func TestSilenceDuration_Duration(t *testing.T) {
	tests := []struct {
		input   v1alpha2.SilenceDuration
		want    time.Duration
		wantErr bool
	}{
		{input: "30m", want: 30 * time.Minute},
		{input: "1h", want: time.Hour},
		{input: "24h", want: 24 * time.Hour},
		{input: "1d", want: 24 * time.Hour},
		{input: "7d", want: 7 * 24 * time.Hour},
		{input: "1w", want: 7 * 24 * time.Hour},
		{input: "2w", want: 14 * 24 * time.Hour},
		{input: "1d12h", want: 36 * time.Hour},
		{input: "2w3d", want: (14 + 3) * 24 * time.Hour},
		{input: "1w2d3h30m", want: (7+2)*24*time.Hour + 3*time.Hour + 30*time.Minute},
		{input: "168h", want: 168 * time.Hour},
		{input: "", wantErr: true},
		{input: "invalid", wantErr: true},
		{input: "1x", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(string(tc.input), func(t *testing.T) {
			got, err := tc.input.Duration()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
