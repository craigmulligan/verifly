package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"google.golang.org/appengine" // Required external App Engine library
	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/urlfetch"
	"log"
	"net"
	"net/http"
	"time"
)

type Record struct {
	Domain      string `json:"domain"`
	Challenge   string `json:"challenge"`
	Verified    bool   `json:"verified"`
	CallbackUrl string `json:"callback_url"`
}

func lookupRecord(record *Record) bool {
	txtrecords, err := net.LookupTXT(record.Domain)

	if err != nil {
		log.Fatalln(err)
		return false
	}

	for _, txt := range txtrecords {
		if txt == record.Challenge {
			return true
		}

	}

	return false
}

func notify(rw *http.Request, payload Record) (*http.Response, error) {
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(payload)

	req, err := http.NewRequest("POST", payload.CallbackUrl, body)

	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("Content-Type", "application/json")

	ctx := appengine.NewContext(rw)
	client := urlfetch.Client(ctx)

	return client.Do(req)
}

func createChallenge() (string, error) {
	id, err := uuid.NewUUID()
	return id.String(), err
}

func PostTask(record Record) *taskqueue.Task {
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	payload, _ := json.Marshal(record)
	ageLimit, _ := time.ParseDuration("20m")
	minBackoff, _ := time.ParseDuration("5s")

	return &taskqueue.Task{
		Path:    "/challenge",
		Payload: payload,
		Header:  h,
		Method:  "POST",
		RetryOptions: &taskqueue.RetryOptions{
			AgeLimit:   ageLimit,
			MinBackoff: minBackoff,
		},
	}
}

func scheduleTask(record Record, ctx context.Context) (*taskqueue.Task, error) {
	t := PostTask(record)

	return taskqueue.Add(ctx, t, "")
}

func worker(rw http.ResponseWriter, req *http.Request) {
	ctx := appengine.NewContext(req)

	decoder := json.NewDecoder(req.Body)
	var record Record

	err := decoder.Decode(&record)
	if err != nil {
		log.Fatalln(err)
	}

	if record.Domain == "" {
		challenge, _ := createChallenge()
		record.Challenge = challenge
	}

	_, err = scheduleTask(record, ctx)
	if err != nil {
		log.Fatalln(err)
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(record)
}

func challenge(rw http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var record Record

	err := decoder.Decode(&record)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Worker challenge", record.Challenge)
	record.Verified = lookupRecord(&record)

	// Finished
	if record.Verified {
		res, err := notify(req, record)
		if err != nil {
			rw.WriteHeader(res.StatusCode)
			return
		}
		log.Printf("Worker complete", record.Challenge)
		rw.WriteHeader(http.StatusOK)
		return
	}

	rw.WriteHeader(http.StatusNotFound)
	json.NewEncoder(rw).Encode(record)
}

func main() {
	http.HandleFunc("/worker", worker)
	http.HandleFunc("/challenge", challenge)
	appengine.Main()
}
