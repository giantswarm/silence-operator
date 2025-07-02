package alertmanager

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/silence-operator/api/v1alpha1"
	"github.com/giantswarm/silence-operator/pkg/config"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      config.Config
		expectError bool
	}{
		{
			name: "valid config",
			config: config.Config{
				Address:        "http://localhost:9093",
				Authentication: false,
				BearerToken:    "",
				TenantId:       "",
			},
			expectError: false,
		},
		{
			name: "valid config with auth",
			config: config.Config{
				Address:        "http://localhost:9093",
				Authentication: true,
				BearerToken:    "test-token",
				TenantId:       "test-tenant",
			},
			expectError: false,
		},
		{
			name: "empty address",
			config: config.Config{
				Address: "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am, err := New(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, am)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, am)
				assert.Equal(t, tt.config.Address, am.address)
				assert.Equal(t, tt.config.Authentication, am.authentication)
				assert.Equal(t, tt.config.BearerToken, am.token)
				assert.Equal(t, tt.config.TenantId, am.tenantId)
			}
		})
	}
}

func TestSilenceComment(t *testing.T) {
	silence := &v1alpha1.Silence{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-silence",
		},
	}

	comment := SilenceComment(silence)
	expected := "silence-operator-test-silence"

	assert.Equal(t, expected, comment)
}

func TestSilenceEndsAt(t *testing.T) {
	baseTime := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		silence      *v1alpha1.Silence
		expectedTime time.Time
		expectError  bool
	}{
		{
			name: "no annotation - use creation timestamp + 100 years",
			silence: &v1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: baseTime},
				},
			},
			expectedTime: baseTime.AddDate(100, 0, 0),
			expectError:  false,
		},
		{
			name: "valid RFC3339 annotation",
			silence: &v1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: baseTime},
					Annotations: map[string]string{
						ValidUntilAnnotationName: "2023-12-31T23:59:59Z",
					},
				},
			},
			expectedTime: time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
			expectError:  false,
		},
		{
			name: "valid date-only annotation (legacy format)",
			silence: &v1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: baseTime},
					Annotations: map[string]string{
						ValidUntilAnnotationName: "2023-12-31",
					},
				},
			},
			expectedTime: time.Date(2023, 12, 31, 8, 0, 0, 0, time.UTC), // Shifted to 8am UTC
			expectError:  false,
		},
		{
			name: "invalid date annotation",
			silence: &v1alpha1.Silence{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: baseTime},
					Annotations: map[string]string{
						ValidUntilAnnotationName: "invalid-date",
					},
				},
			},
			expectedTime: time.Time{},
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SilenceEndsAt(tt.silence)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid-date")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedTime, result)
			}
		})
	}
}

func TestAlertmanager_ListSilences(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/silences", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`[
			{
				"id": "test-id-1",
				"comment": "test-comment-1",
				"createdBy": "silence-operator",
				"startsAt": "2023-01-01T10:00:00Z",
				"endsAt": "2023-01-01T11:00:00Z",
				"matchers": [
					{
						"name": "alertname",
						"value": "test-alert",
						"isRegex": false,
						"isEqual": true
					}
				],
				"status": {
					"state": "active"
				}
			},
			{
				"id": "test-id-2",
				"comment": "test-comment-2",
				"createdBy": "silence-operator",
				"startsAt": "2023-01-01T10:00:00Z",
				"endsAt": "2023-01-01T11:00:00Z",
				"matchers": [],
				"status": {
					"state": "expired"
				}
			}
		]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	config := config.Config{
		Address: server.URL,
	}
	am, err := New(config)
	require.NoError(t, err)

	silences, err := am.ListSilences("")

	assert.NoError(t, err)
	assert.Len(t, silences, 1) // Only non-expired silences should be returned
	assert.Equal(t, "test-id-1", silences[0].ID)
	assert.Equal(t, "test-comment-1", silences[0].Comment)
}

func TestAlertmanager_GetSilenceByComment(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/silences", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`[
			{
				"id": "test-id-1",
				"comment": "silence-operator-test-silence",
				"createdBy": "silence-operator",
				"startsAt": "2023-01-01T10:00:00Z",
				"endsAt": "2023-01-01T11:00:00Z",
				"matchers": [],
				"status": {
					"state": "active"
				}
			}
		]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	config := config.Config{
		Address: server.URL,
	}
	am, err := New(config)
	require.NoError(t, err)

	tests := []struct {
		name        string
		comment     string
		expectError bool
		errorType   error
	}{
		{
			name:        "found silence",
			comment:     "silence-operator-test-silence",
			expectError: false,
		},
		{
			name:        "silence not found",
			comment:     "non-existent-comment",
			expectError: true,
			errorType:   ErrSilenceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			silence, err := am.GetSilenceByComment(tt.comment, "")

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.errorType)
				assert.Nil(t, silence)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, silence)
				assert.Equal(t, "test-id-1", silence.ID)
				assert.Equal(t, tt.comment, silence.Comment)
			}
		})
	}
}

func TestAlertmanager_CreateSilence(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/silences", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := config.Config{
		Address: server.URL,
	}
	am, err := New(config)
	require.NoError(t, err)

	silence := &Silence{
		Comment:   "test-comment",
		CreatedBy: CreatedBy,
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(time.Hour),
		Matchers: []Matcher{
			{
				Name:    "alertname",
				Value:   "test-alert",
				IsRegex: false,
				IsEqual: true,
			},
		},
	}

	err = am.CreateSilence(silence, "")
	assert.NoError(t, err)
}

func TestAlertmanager_UpdateSilence(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/silences", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := config.Config{
		Address: server.URL,
	}
	am, err := New(config)
	require.NoError(t, err)

	silence := &Silence{
		ID:        "test-id",
		Comment:   "test-comment",
		CreatedBy: CreatedBy,
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(time.Hour),
		Matchers: []Matcher{
			{
				Name:    "alertname",
				Value:   "test-alert",
				IsRegex: false,
				IsEqual: true,
			},
		},
	}

	err = am.UpdateSilence(silence, "")
	assert.NoError(t, err)
}

func TestAlertmanager_UpdateSilence_MissingID(t *testing.T) {
	config := config.Config{
		Address: "http://localhost:9093",
	}
	am, err := New(config)
	require.NoError(t, err)

	silence := &Silence{
		Comment:   "test-comment",
		CreatedBy: CreatedBy,
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(time.Hour),
	}

	err = am.UpdateSilence(silence, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing ID")
}

func TestAlertmanager_DeleteSilenceByID(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/silence/test-id", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := config.Config{
		Address: server.URL,
	}
	am, err := New(config)
	require.NoError(t, err)

	err = am.DeleteSilenceByID("test-id", "")
	assert.NoError(t, err)
}

func TestAlertmanager_DeleteSilenceByComment(t *testing.T) {
	callCount := 0
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call - list silences
			assert.Equal(t, "/api/v2/silences", r.URL.Path)
			assert.Equal(t, "GET", r.Method)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`[
				{
					"id": "test-id",
					"comment": "test-comment",
					"createdBy": "silence-operator",
					"startsAt": "2023-01-01T10:00:00Z",
					"endsAt": "2023-01-01T11:00:00Z",
					"matchers": [],
					"status": {
						"state": "active"
					}
				}
			]`))
			require.NoError(t, err)
		} else {
			// Second call - delete silence
			assert.Equal(t, "/api/v2/silence/test-id", r.URL.Path)
			assert.Equal(t, "DELETE", r.Method)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	config := config.Config{
		Address: server.URL,
	}
	am, err := New(config)
	require.NoError(t, err)

	err = am.DeleteSilenceByComment("test-comment", "")
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount) // Should have made both calls
}

func TestAlertmanager_WithAuthentication(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authentication header
		auth := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer test-token", auth)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`[]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	config := config.Config{
		Address:        server.URL,
		Authentication: true,
		BearerToken:    "test-token",
	}
	am, err := New(config)
	require.NoError(t, err)

	_, err = am.ListSilences()
	assert.NoError(t, err)
}

func TestAlertmanager_WithTenantID(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check tenant header
		tenant := r.Header.Get("X-Scope-OrgID")
		assert.Equal(t, "test-tenant", tenant)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`[]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	config := config.Config{
		Address:  server.URL,
		TenantId: "test-tenant",
	}
	am, err := New(config)
	require.NoError(t, err)

	_, err = am.ListSilences()
	assert.NoError(t, err)
}

// Tenant-aware method tests

func TestAlertmanager_CreateSilenceWithTenant(t *testing.T) {
	// Create test server that checks for X-Scope-OrgID header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/silences", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-tenant", r.Header.Get("X-Scope-OrgID"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := config.Config{
		Address: server.URL,
	}
	am, err := New(config)
	require.NoError(t, err)

	silence := &Silence{
		Comment:   "test-comment",
		CreatedBy: CreatedBy,
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(time.Hour),
		Matchers: []Matcher{
			{
				Name:    "alertname",
				Value:   "test-alert",
				IsRegex: false,
				IsEqual: true,
			},
		},
	}

	err = am.CreateSilenceWithTenant(silence, "test-tenant")
	assert.NoError(t, err)
}

func TestAlertmanager_ListSilencesWithTenant(t *testing.T) {
	// Create test server that checks for X-Scope-OrgID header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/silences", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "test-tenant", r.Header.Get("X-Scope-OrgID"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`[
			{
				"id": "test-id-1",
				"comment": "test-comment-1",
				"createdBy": "silence-operator",
				"startsAt": "2023-01-01T10:00:00Z",
				"endsAt": "2023-01-01T11:00:00Z",
				"matchers": [],
				"status": {
					"state": "active"
				}
			}
		]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	config := config.Config{
		Address: server.URL,
	}
	am, err := New(config)
	require.NoError(t, err)

	silences, err := am.ListSilencesWithTenant("test-tenant")
	assert.NoError(t, err)
	assert.Len(t, silences, 1)
	assert.Equal(t, "test-id-1", silences[0].ID)
}

func TestAlertmanager_GetSilenceByCommentWithTenant(t *testing.T) {
	// Create test server that checks for X-Scope-OrgID header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/silences", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "test-tenant", r.Header.Get("X-Scope-OrgID"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`[
			{
				"id": "test-id-1",
				"comment": "silence-operator-test-silence",
				"createdBy": "silence-operator",
				"startsAt": "2023-01-01T10:00:00Z",
				"endsAt": "2023-01-01T11:00:00Z",
				"matchers": [],
				"status": {
					"state": "active"
				}
			}
		]`))
		require.NoError(t, err)
	}))
	defer server.Close()

	config := config.Config{
		Address: server.URL,
	}
	am, err := New(config)
	require.NoError(t, err)

	silence, err := am.GetSilenceByCommentWithTenant("silence-operator-test-silence", "test-tenant")
	assert.NoError(t, err)
	assert.NotNil(t, silence)
	assert.Equal(t, "test-id-1", silence.ID)
	assert.Equal(t, "silence-operator-test-silence", silence.Comment)
}

func TestAlertmanager_DeleteSilenceByIDWithTenant(t *testing.T) {
	// Create test server that checks for X-Scope-OrgID header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/silence/test-id", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "test-tenant", r.Header.Get("X-Scope-OrgID"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := config.Config{
		Address: server.URL,
	}
	am, err := New(config)
	require.NoError(t, err)

	err = am.DeleteSilenceByIDWithTenant("test-id", "test-tenant")
	assert.NoError(t, err)
}

func TestAlertmanager_TenantPrecedence(t *testing.T) {
	// Test that parameter tenant takes precedence over instance tenant
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should get the parameter tenant, not the instance tenant
		assert.Equal(t, "param-tenant", r.Header.Get("X-Scope-OrgID"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	config := config.Config{
		Address:  server.URL,
		TenantId: "instance-tenant", // This should be overridden
	}
	am, err := New(config)
	require.NoError(t, err)

	_, err = am.ListSilencesWithTenant("param-tenant")
	assert.NoError(t, err)
}

func TestAlertmanager_BackwardCompatibilityUsesInstanceTenant(t *testing.T) {
	// Test that old methods still use the instance tenantId
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "instance-tenant", r.Header.Get("X-Scope-OrgID"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	config := config.Config{
		Address:  server.URL,
		TenantId: "instance-tenant",
	}
	am, err := New(config)
	require.NoError(t, err)

	// Old method should still use instance tenant
	_, err = am.ListSilences()
	assert.NoError(t, err)
}
