// Provides a basic proxy over the MailerLite subscription API
//
// Copyright © 2023 Matt Brown. MIT Licensed.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const ML_URL = "https://connect.mailerlite.com/api/subscribers"

type Config struct {
	Port string

	ML_group  int
	ML_apikey string
	Referer   string
	Host      string

	Valid bool
}

func (c *Config) Load() {
	c.Port = os.Getenv("PORT")
	if c.Port == "" {
		c.Port = "8080"
	}

	c.ML_apikey = os.Getenv("ML_APIKEY")
	v, err := strconv.Atoi(os.Getenv("ML_GROUP"))
	if err != nil {
		fmt.Printf("ML_GROUP is invalid: %v\n", err)
	} else {
		c.ML_group = v
	}
	c.Referer = os.Getenv("REFERER")
	c.Host = os.Getenv("HOST")

	if c.ML_apikey != "" && c.ML_group != 0 && c.Referer != "" && c.Host != "" {
		c.Valid = true
	}
}

var GlobalConfig Config

type SubRequest struct {
	Email      string `json:"email"`
	Groups     []int  `json:"groups"`
	Status     string `json:"status"`
	IP_address string `json:"ip_address"`
}

// Helper to render an error as a JSON response
func RenderJSONError(err error, code int, w http.ResponseWriter) {
	RenderJSON(map[string]any{"error": err.Error()}, code, w)
}

func RenderJSON(d map[string]any, code int, w http.ResponseWriter) {
	j, err := json.Marshal(d)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	w.Write(j)
}

func HandleSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Add("Location", GlobalConfig.Referer)
		w.WriteHeader(http.StatusFound)
		return
	}
	if !GlobalConfig.Valid {
		RenderJSONError(errors.New("service unavailable"), http.StatusServiceUnavailable, w)
		return
	}
	/*
	   TODO: Work out why this isn't working on fly.io
	   host := r.Header.Get("Host")
	   auth := r.Header.Get(":authority")
	   fmt.Println("Host", host, " auth ", auth)

	   Something to do with HTTP2 I think, maybe fly is not translating the pseudo-header to Host when
	   forwarding the connection on?

	   host, _, _ := net.SplitHostPort(r.Host)
	   if host == "" {
	           host = r.URL.Host
	           fmt.Printf("Host header not set, using %s from URL\n", host)
	   }
	   if host != "subscribe.mattb.nz" {
	           RenderJSONError(fmt.Errorf("bad host - %s", host), http.StatusBadRequest, w)
	           return
	   }
	*/

	if r.Header.Get("X-Forwarded-SSL") != "on" {
		RenderJSONError(errors.New("ssl required"), http.StatusBadRequest, w)
		return
	}

	referer := r.Header.Get("Referer")
	fmt.Println("Referer", referer)

	if !strings.HasPrefix(referer, GlobalConfig.Referer) {
		log.Printf("ERROR: %s is not from %s\n", referer, GlobalConfig.Referer)
		RenderJSONError(errors.New("invalid referer"), http.StatusBadRequest, w)
		return
	}

	sr := SubRequest{}
	err := json.NewDecoder(r.Body).Decode(&sr)
	if err != nil {
		RenderJSONError(errors.New("could not parse subscription request"), http.StatusBadRequest, w)
		return
	}
	if sr.Email == "" {
		RenderJSONError(errors.New("email is required"), http.StatusBadRequest, w)
		return
	}
	sr.Groups = []int{GlobalConfig.ML_group}
	sr.Status = "unconfirmed"
	sr.IP_address = r.Header.Get("Fly-Client-IP")

	jb, err := json.Marshal(sr)
	if err != nil {
		RenderJSONError(errors.New("could not marshall subscription request"), http.StatusInternalServerError, w)
	}
	fmt.Println(string(jb))
	request, error := http.NewRequest("POST", ML_URL, bytes.NewBuffer(jb))
	if error != nil {
		RenderJSONError(errors.New("could not post subscription request"), http.StatusInternalServerError, w)
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", GlobalConfig.ML_apikey))

	client := &http.Client{}
	response, error := client.Do(request)
	if error != nil {
		panic(error)
	}
	defer response.Body.Close()

	fmt.Println("response Status:", response.Status)
	fmt.Println("response Headers:", response.Header)
	body, _ := io.ReadAll(response.Body)
	fmt.Println("response Body:", string(body))
	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusCreated {
		RenderJSON(map[string]any{"status": "subscribed"}, http.StatusOK, w)
	} else {
		RenderJSONError(errors.New("subscription failed"), response.StatusCode, w)
	}
}

func main() {
	GlobalConfig.Load()
	if !GlobalConfig.Valid {
		log.Println("WARNING: invalid config - see /healthz for details!")
	}

	// Handle requests
	http.HandleFunc("/", HandleSubscribe)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if GlobalConfig.Valid {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("all good"))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			if GlobalConfig.ML_apikey == "" {
				w.Write([]byte("missing apikey\n"))
			}
			if GlobalConfig.ML_group == -1 {
				w.Write([]byte("missing group\n"))
			}
			if GlobalConfig.Referer == "" {
				w.Write([]byte("missing referer\n"))
			}
			if GlobalConfig.Host == "" {
				w.Write([]byte("missing host\n"))
			}
		}
	})
	log.Println("listening on", GlobalConfig.Port)
	log.Fatal(http.ListenAndServe(":"+GlobalConfig.Port, nil))
}
