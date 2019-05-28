package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"google.golang.org/appengine" // Required external App Engine library
	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/urlfetch"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Record struct {
	Domain      string `json:"domain"`
	Challenge   string `json:"challenge"`
	Verified    bool   `json:"verified"`
	CallbackUrl string `json:"callback_url"`
}

type CloudFlareRecord struct {
	Data string `json:"data"`
}

type CloudFlareRes struct {
	Answer []CloudFlareRecord `json:"Answer"`
}

func lookupRecord(record *Record) (bool, error) {
	txtrecords, err := net.LookupTXT(record.Domain)

	if err != nil {
		return false, err
	}

	for _, txt := range txtrecords {
		if txt == record.Challenge {
			return true, nil
		}

	}

	return false, nil
}

// We use http instead of udp dns query because appengine standard has specific requirements on socket use.
func lookupRecordHttp(ctx context.Context, payload Record) (bool, error) {
	client := urlfetch.Client(ctx)
	req, err := http.NewRequest("GET", "https://cloudflare-dns.com/dns-query", nil)
	req.Header.Add("accept", "application/dns-json")

	// or you can create new url.Values struct and encode that like so
	q := url.Values{}
	q.Add("name", "transparently.app")
	q.Add("type", "TXT")

	req.URL.RawQuery = q.Encode()
	log.Printf(req.URL.RawQuery)

	if err != nil {
		return false, err
	}

	res, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	var records CloudFlareRes
	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return false, err
	}

	err = json.Unmarshal(body, &records)
	if err != nil {
		return false, err
	}

	for _, txt := range records.Answer {
		s := txt.Data
		s = strings.TrimSuffix(s, "\"")
		s = strings.TrimPrefix(s, "\"")
		if s == payload.Challenge {
			return true, nil
		}
	}

	return false, nil

	if err != nil {
		return false, err
	}
	return true, nil
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
	return "verifly-site-verification=" + id.String(), err
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

	if record.Challenge == "" {
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
	ctx := appengine.NewContext(req)

	decoder := json.NewDecoder(req.Body)
	var record Record

	err := decoder.Decode(&record)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Worker challenge", record.Challenge)

	verified, err := lookupRecordHttp(ctx, record)

	if err != nil {
		log.Printf("err", err)
		rw.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(rw).Encode(err)
		return
	}

	record.Verified = verified
	// Finished
	if record.Verified {
		_, err := notify(req, record)
		if err != nil {
			log.Printf("Could not notify caller %+v", record.Challenge)
			rw.WriteHeader(502)
			json.NewEncoder(rw).Encode(record)
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
