package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/danielbusnz-lgtm/threadsmith/internal/middleware"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "could not read body", http.StatusBadRequest)
			return
		}
		w.Write(body)
	})

	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "could not read body", http.StatusBadRequest)
			return
		}

		upstream, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			http.Error(w, "could not build upstream request", http.StatusInternalServerError)
			return
		}
		upstream.Header.Set("Content-Type", "application/json")
		upstream.Header.Set("Authorization", "Bearer "+os.Getenv("OPENAI_API_KEY"))

		resp, err := http.DefaultClient.Do(upstream)
		if err != nil {
			http.Error(w, "upstream request failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "could not read upstream response", http.StatusBadGateway)
			return
		}

		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	})

	addr := ":8080"
	log.Info("threadsmith gateway listening", "addr", addr)
	if err := http.ListenAndServe(addr, middleware.Logging(mux)); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}
