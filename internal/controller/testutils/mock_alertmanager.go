package testutils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/giantswarm/silence-operator/pkg/alertmanager"
	"github.com/giantswarm/silence-operator/pkg/config"
)

// MockAlertManagerServer provides a mock AlertManager HTTP server for testing
type MockAlertManagerServer struct {
	server   *httptest.Server
	silences map[string]*alertmanager.Silence
	mu       sync.RWMutex
}

// NewMockAlertManagerServer creates a new mock AlertManager server
func NewMockAlertManagerServer() *MockAlertManagerServer {
	mock := &MockAlertManagerServer{
		silences: make(map[string]*alertmanager.Silence),
	}

	// Create HTTP test server with mock handlers
	mux := http.NewServeMux()

	// Handle GET /api/v2/silences - list silences
	mux.HandleFunc("/api/v2/silences", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			mock.handleListSilences(w)
		case http.MethodPost:
			mock.handleCreateSilence(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Handle DELETE /api/v2/silence/{id} - delete silence by ID
	mux.HandleFunc("/api/v2/silence/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			mock.handleDeleteSilence(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mock.server = httptest.NewServer(mux)
	return mock
}

// GetAlertManager returns a real AlertManager configured to use the mock server
func (m *MockAlertManagerServer) GetAlertManager() (*alertmanager.AlertManager, error) {
	config := config.Config{
		Address:        m.server.URL,
		Authentication: false,
	}
	return alertmanager.New(config)
}

// Close shuts down the mock server
func (m *MockAlertManagerServer) Close() {
	m.server.Close()
}

// AddSilence adds a silence to the mock server's state
func (m *MockAlertManagerServer) AddSilence(silence *alertmanager.Silence) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if silence.ID == "" {
		silence.ID = "mock-id-" + silence.Comment
	}
	m.silences[silence.ID] = silence
}

// GetSilences returns all silences from the mock server
func (m *MockAlertManagerServer) GetSilences() []*alertmanager.Silence {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var silences []*alertmanager.Silence
	for _, silence := range m.silences {
		silences = append(silences, silence)
	}
	return silences
}

func (m *MockAlertManagerServer) handleListSilences(w http.ResponseWriter) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var silences []alertmanager.Silence
	for _, silence := range m.silences {
		// Only return non-expired silences (like the real AlertManager)
		if silence.Status == nil || silence.Status.State != alertmanager.SilenceStateExpired {
			silences = append(silences, *silence)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(silences); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (m *MockAlertManagerServer) handleCreateSilence(w http.ResponseWriter, r *http.Request) {
	var silence alertmanager.Silence
	if err := json.NewDecoder(r.Body).Decode(&silence); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ID if not provided (new silence)
	if silence.ID == "" {
		silence.ID = "mock-id-" + silence.Comment
	}

	m.silences[silence.ID] = &silence

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"silenceID": silence.ID}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (m *MockAlertManagerServer) handleDeleteSilence(w http.ResponseWriter, r *http.Request) {
	// Extract silence ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/silence/")
	silenceID := strings.Split(path, "/")[0]

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.silences[silenceID]; !exists {
		http.Error(w, "Silence not found", http.StatusNotFound)
		return
	}

	delete(m.silences, silenceID)
	w.WriteHeader(http.StatusOK)
}
