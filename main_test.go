package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/klyve/cloud-oblig2/currency"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var respRec *httptest.ResponseRecorder
var m *mux.Router

func setup() {
	//mux router with added question routes
	m = mux.NewRouter()
	session, err := mgo.Dial("localhost")
	if err != nil {
		log.Fatal("Could not connect to the database")
	}

	db := session.DB("oblig2-test")
	Init(db, m)

	//The response recorder used to record HTTP responses
	respRec = httptest.NewRecorder()
	insertDummyData()
}

func insertDummyData() {
	path := "./data/test.json"
	file, e := ioutil.ReadFile(path)
	if e != nil {
		fmt.Printf("File error: %v\n", e)
	}
	var latest DataList
	jsonError := json.Unmarshal(file, &latest)
	if jsonError != nil {
		fmt.Printf("Failed to parse json: %v\n", jsonError)
	}

	collection := database.C("currency")
	collection.RemoveAll(nil)
	for i := 1; i <= 7; i++ {
		data := currency.Currency{
			ID:    bson.NewObjectId(),
			Base:  latest.Base,
			Date:  latest.Date,
			Rates: latest.Rates,
		}
		err := collection.Insert(&data)
		if err != nil {
			fmt.Printf("Error %v", err)
		}
	}
}

type WebHook struct {
	ID              bson.ObjectId `json:"id" bson:"_id"`
	WebhookURL      string        `json:"webhookURL"`
	BaseCurrency    string        `json:"baseCurrency"`
	TargetCurrency  string        `json:"targetCurrency"`
	MinTriggerValue float32       `json:"minTriggerValue"`
	MaxTriggerValue float32       `json:"maxTriggerValue"`
}

func TestSetup(t *testing.T) {
	setup()
}

func TestWebhook(t *testing.T) {
	setup()
	collection := database.C("webhooks")
	collection.RemoveAll(nil)
	hook := WebHook{
		WebhookURL:      "HelloWorld",
		BaseCurrency:    "NOK",
		TargetCurrency:  "SEK",
		MinTriggerValue: 1.2,
		MaxTriggerValue: 1.5,
	}
	data := new(bytes.Buffer)
	json.NewEncoder(data).Encode(hook)
	req, err := http.NewRequest("POST", "/", data)
	if err != nil {
		t.Fatal("Creating 'POST ' request failed!")
	}

	m.ServeHTTP(respRec, req)

	if respRec.Code != 200 {
		t.Fatal("Server error: Returned ", respRec.Code, " instead of ", 200)
	}

	str := fmt.Sprintf("%s", respRec.Body)
	respRec = httptest.NewRecorder()
	var url string
	url = "/" + str
	req2, err2 := http.NewRequest("GET", url, nil)
	if err2 != nil {
		t.Fatal("Creating 'POST ' request failed!")
	}

	m.ServeHTTP(respRec, req2)

	if respRec.Code != 200 {
		t.Fatal("Server error: Returned ", respRec.Code, " instead of ", 200)
	}

	var wh WebHook
	decoder := json.NewDecoder(respRec.Body)
	err3 := decoder.Decode(&wh)

	if err3 != nil {
		fmt.Println(err3)
	}

	if wh.ID.Hex() != str {
		t.Fatal("It does not match")
	}
	if wh.BaseCurrency != "NOK" {
		t.Fatal("Basecurrency is not NOK")
	}
	if wh.TargetCurrency != "SEK" {
		t.Fatal("Target is not SEK")
	}

	respRec = httptest.NewRecorder()

	req3, err4 := http.NewRequest("DELETE", url, nil)
	if err4 != nil {
		t.Fatal("Creating 'DELETE ' request failed!")
	}

	m.ServeHTTP(respRec, req3)

	if respRec.Code != 200 {
		t.Fatal("Server error: Returned ", respRec.Code, " instead of ", 200)
	}

	respRec = httptest.NewRecorder()

	req4, err6 := http.NewRequest("DELETE", url, nil)
	if err6 != nil {
		t.Fatal("Creating 'DELETE ' request failed!")
	}

	m.ServeHTTP(respRec, req4)

	if respRec.Code != 404 {
		t.Fatal("Server error: Returned ", respRec.Code, " instead of ", 404)
	}

}

func TestWebhookData(t *testing.T) {
	setup()
	hook := LatestRates{
		BaseCurrency:   "NOK",
		TargetCurrency: "SEK",
	}
	data := new(bytes.Buffer)
	json.NewEncoder(data).Encode(hook)
	req, err := http.NewRequest("POST", "/latest", data)
	if err != nil {
		t.Fatal("Creating 'POST ' request failed!")
	}

	m.ServeHTTP(respRec, req)

	if respRec.Code != 200 {
		t.Fatal("Server error: Returned ", respRec.Code, " instead of ", 200)
	}

}

func TestWebhookAvgData(t *testing.T) {
	setup()
	hook := LatestRates{
		BaseCurrency:   "NOK",
		TargetCurrency: "SEK",
	}
	data := new(bytes.Buffer)
	json.NewEncoder(data).Encode(hook)
	req, err := http.NewRequest("POST", "/average", data)
	if err != nil {
		t.Fatal("Creating 'POST ' request failed!")
	}

	m.ServeHTTP(respRec, req)

	if respRec.Code != 200 {
		t.Fatal("Server error: Returned ", respRec.Code, " instead of ", 200)
	}

}
