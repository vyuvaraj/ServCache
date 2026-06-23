package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"servcache/pkg/cache"
	"servcache/pkg/otel"
)

type Server struct {
	cache cache.Cache
}

func NewServer(c cache.Cache) *Server {
	return &Server{cache: c}
}

type SetRequest struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	TTL   string      `json:"ttl,omitempty"`
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("/api/cache", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPost:
			s.handleSet(w, req)
		case http.MethodDelete:
			s.handleClear(w, req)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/cache/", func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 3 {
			http.Error(w, "Cache key required", http.StatusBadRequest)
			return
		}
		key := parts[2]

		switch req.Method {
		case http.MethodGet:
			s.handleGet(w, req, key)
		case http.MethodDelete:
			s.handleDelete(w, req, key)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	return mux
}

func (s *Server) handleGet(w http.ResponseWriter, req *http.Request, key string) {
	traceparent := req.Header.Get("traceparent")
	span := otel.StartSpan(fmt.Sprintf("servcache:GET %s", key), traceparent)

	val, found, err := s.cache.Get(key)
	
	if span != nil {
		otel.EndSpan(span, err, map[string]interface{}{
			"cache.key":   key,
			"cache.hit":   found,
			"cache.error": err != nil,
		})
	}

	if err != nil {
		http.Error(w, "Cache Read Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !found {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"key":   key,
		"value": val,
	})
}

func (s *Server) handleSet(w http.ResponseWriter, req *http.Request) {
	var body SetRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "Malformed JSON", http.StatusBadRequest)
		return
	}

	if body.Key == "" || body.Value == nil {
		http.Error(w, "Key and Value are required", http.StatusBadRequest)
		return
	}

	var ttl time.Duration
	if body.TTL != "" {
		parsed, err := time.ParseDuration(body.TTL)
		if err != nil {
			http.Error(w, "Invalid TTL format: "+err.Error(), http.StatusBadRequest)
			return
		}
		ttl = parsed
	}

	traceparent := req.Header.Get("traceparent")
	span := otel.StartSpan(fmt.Sprintf("servcache:SET %s", body.Key), traceparent)

	err := s.cache.Set(body.Key, body.Value, ttl)

	if span != nil {
		otel.EndSpan(span, err, map[string]interface{}{
			"cache.key":   body.Key,
			"cache.ttl":   body.TTL,
			"cache.error": err != nil,
		})
	}

	if err != nil {
		http.Error(w, "Cache Write Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) handleDelete(w http.ResponseWriter, req *http.Request, key string) {
	traceparent := req.Header.Get("traceparent")
	span := otel.StartSpan(fmt.Sprintf("servcache:DELETE %s", key), traceparent)

	err := s.cache.Delete(key)

	if span != nil {
		otel.EndSpan(span, err, map[string]interface{}{
			"cache.key":   key,
			"cache.error": err != nil,
		})
	}

	if err != nil {
		http.Error(w, "Cache Delete Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) handleClear(w http.ResponseWriter, req *http.Request) {
	traceparent := req.Header.Get("traceparent")
	span := otel.StartSpan("servcache:CLEAR", traceparent)

	err := s.cache.Clear()

	if span != nil {
		otel.EndSpan(span, err, map[string]interface{}{
			"cache.error": err != nil,
		})
	}

	if err != nil {
		http.Error(w, "Cache Clear Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}
