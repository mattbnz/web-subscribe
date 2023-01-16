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
)

const ML_URL = "https://connect.mailerlite.com/api/subscribers"
const ML_GROUP_ALL = 77434585283036318

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
		w.Header().Add("Location", "https://wwww.mattb.nz/")
		w.WriteHeader(http.StatusFound)
		return
	}
	/*
		TODO: Work out why this isn't working on fly.io
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
	sr.Groups = []int{ML_GROUP_ALL}
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
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("ML_APIKEY")))

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
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", HandleSubscribe)

	log.Println("listening on", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}