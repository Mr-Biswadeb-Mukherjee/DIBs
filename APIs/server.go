// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package apis

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var apiISTLocation = loadAPIISTLocation()

type Server struct {
	sessions     *SessionManager
	keys         APIKeyPair
	apiKeyHeader string
	details      *DetailsService
	limiter      *EndpointRateLimiter
}

func NewRouter(
	sessions *SessionManager,
	keys APIKeyPair,
	contractPath string,
) (http.Handler, error) {
	path := strings.TrimSpace(contractPath)
	if path == "" {
		path = DefaultEndpointContractPath
	}
	contract, err := LoadEndpointContract(path)
	if err != nil {
		return nil, err
	}

	server := &Server{
		sessions:     sessions,
		keys:         keys,
		apiKeyHeader: contract.APIKeyHeader,
		details:      NewDetailsService(defaultDetailsOutputDir),
		limiter:      NewEndpointRateLimiter(),
	}
	mux, err := server.buildMux(contract)
	if err != nil {
		return nil, err
	}
	return withCORS(mux, contract.APIKeyHeader), nil
}

func (s *Server) buildMux(contract EndpointContract) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	handlers := s.routeHandlers()
	for _, route := range contract.Routes {
		handler, ok := handlers[route.Name]
		if !ok {
			return nil, fmt.Errorf("missing handler for route: %s", route.Name)
		}
		withAuth := s.applyAuth(route.Auth, handler)
		mux.HandleFunc(routePattern(route), s.applyRoutePolicy(route, withAuth))
	}
	return mux, nil
}

func (s *Server) routeHandlers() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"health":  s.handleHealth,
		"start":   s.handleStart,
		"stop":    s.handleStop,
		"status":  s.handleStatus,
		"metrics": s.handleMetrics,
		"events":  s.handleEvents,
		"details": s.handleDetails,
	}
}

func routePattern(route RouteSpec) string {
	return route.Method + " " + route.Path
}

func (s *Server) applyAuth(required bool, next http.HandlerFunc) http.HandlerFunc {
	if !required {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.validAPIKey(r) {
			writeError(w, http.StatusUnauthorized, "invalid api key")
			return
		}
		next(w, r)
	}
}

func (s *Server) applyRoutePolicy(route RouteSpec, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.limiter != nil {
			ok, err := s.limiter.Allow(r.Context(), route.Name, route.RatePerSec, route.RatePerMin)
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "rate limiter unavailable")
				return
			}
			if !ok {
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) validAPIKey(r *http.Request) bool {
	return s.keys.ValidatePublic(extractAPIKey(r, s.apiKeyHeader))
}

func extractAPIKey(r *http.Request, header string) string {
	key := strings.TrimSpace(r.Header.Get(header))
	if key != "" {
		return key
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return strings.TrimSpace(r.URL.Query().Get("api_key"))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"service":     "infermal-api",
		"version":     "v3",
		"auth_header": s.apiKeyHeader,
		"auth_mode":   "public_key",
		"timestamp":   formatAPIISTTimestamp(time.Now()),
		"listen_port": 9090,
	})
}

func (s *Server) handleStart(w http.ResponseWriter, _ *http.Request) {
	info, err := s.sessions.StartSession()
	if err == nil {
		writeJSON(w, http.StatusCreated, map[string]any{"session": info})
		return
	}
	if errors.Is(err, ErrSessionRunning) {
		s.writeActiveConflict(w, err)
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func (s *Server) handleStop(w http.ResponseWriter, _ *http.Request) {
	info, err := s.sessions.StopActiveSession()
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	statusCode := http.StatusOK
	if info.Status == StatusStopping {
		statusCode = http.StatusAccepted
	}
	writeJSON(w, statusCode, map[string]any{"session": info})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	info, ok := s.sessions.CurrentSession()
	if !ok {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": info})
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	info, ok := s.sessions.CurrentSession()
	if !ok {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     info.Status,
		"started_at": info.StartedAt,
		"ended_at":   info.EndedAt,
		"error":      info.Error,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessions.CurrentSession(); !ok {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if err := s.streamSessionEvents(w, r); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (s *Server) handleDetails(w http.ResponseWriter, r *http.Request) {
	sections, limit, err := parseDetailsQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.details == nil {
		writeError(w, http.StatusInternalServerError, "details service unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"requested_sections": sections,
		"limit":              limit,
		"details":            s.details.Fetch(sections, limit, s.sessions),
	})
}

func (s *Server) streamSessionEvents(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/x-ndjson")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("streaming not supported")
	}

	encoder := json.NewEncoder(w)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		if done := writeSessionEvent(encoder, s.sessions, flusher); done {
			return nil
		}
		select {
		case <-r.Context().Done():
			return nil
		case <-ticker.C:
		}
	}
}

func writeSessionEvent(
	encoder *json.Encoder,
	sessions *SessionManager,
	flusher http.Flusher,
) bool {
	info, ok := sessions.CurrentSession()
	if !ok {
		return true
	}
	record := map[string]any{
		"type":          "session.status",
		"timestamp_utc": formatAPIISTTimestamp(time.Now()),
		"session":       info,
	}
	if err := encoder.Encode(record); err != nil {
		return true
	}
	flusher.Flush()
	return info.Status.IsTerminal()
}

func (s *Server) writeActiveConflict(w http.ResponseWriter, sourceErr error) {
	active, ok := s.sessions.CurrentSession()
	if !ok {
		writeError(w, http.StatusConflict, sourceErr.Error())
		return
	}
	writeJSON(w, http.StatusConflict, map[string]any{
		"error":          sourceErr.Error(),
		"active_session": active,
	})
}

func formatAPIISTTimestamp(ts time.Time) string {
	return ts.In(apiISTLocation).Format(time.RFC3339)
}

func loadAPIISTLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err == nil {
		return loc
	}
	return time.FixedZone("IST", 5*60*60+30*60)
}
