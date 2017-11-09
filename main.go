package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/robfig/cron"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

//Data for the Request
type Data struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float32 `json:"rates"`
}

//PostWebhook for incoming Post Requests
type PostWebhook struct {
	ID              bson.ObjectId `json:"id" bson:"_id"`
	WebhookURL      string        `json:"WebhookURL"`
	BaseCurrency    string        `json:"BaseCurrency"`
	TargetCurrency  string        `json:"TargetCurrency"`
	MinTriggerValue float32       `json:"MinTriggerValue"`
	MaxTriggerValue float32       `json:"MaxTriggerValue"`
}

// WebHookInvoked data type
type PostWebhookInvoked struct {
	BaseCurrency    string  `json:"baseCurrency"`
	TargetCurrency  string  `json:"targetCurrency"`
	CurrentRate     float32 `json:"currentRate"`
	MinTriggerValue float32 `json:"minTriggerValue"`
	MaxTriggerValue float32 `json:"maxTriggerValue"`
}
type LatestRates struct {
	BaseCurrency   string `json:"baseCurrency"`
	TargetCurrency string `json:"targetCurrency"`
}

// DataList Holds the currency data
type DataList struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float32 `json:"rates"`
}

// Convertion Holds a single from to currency value
type Convertion struct {
	From      string  `json:"from"`
	FromValue float32 `json:"from_value"`
	To        string  `json:"to"`
	ToValue   float32 `json:"to_value"`
	Rate      float32 `json:"rate"`
}

// Currency holds a currency type
type Currency struct {
	ID    bson.ObjectId      `json:"id" bson:"_id"`
	Base  string             `json:"base" bson:"base"`
	Date  string             `json:"date" bson:"date"`
	Rates map[string]float32 `json:"rates" bson:"rates"`
}

var database *mgo.Database

func main() {

	r := mux.NewRouter()

	session, err := mgo.Dial("localhost")
	if err != nil {
		log.Fatal("Could not connect to the mongoDB server")
	}

	Init(session.DB("cloud2"), r)

	http.ListenAndServe(":8080", r)
}

func Init(db *mgo.Database, r *mux.Router) {
	database = db
	// Cron jobs
	c := cron.New()
	getCronData()
	c.AddFunc("@every 24h", getCronData)
	c.Start()

	r.HandleFunc("/", handlerGet).Methods("GET")
	r.HandleFunc("/", handlerPost).Methods("POST")

	r.HandleFunc("/latest", getLatest).Methods("POST")
	r.HandleFunc("/evaluationtrigger", evaluateTrigger).Methods("POST")
	r.HandleFunc("/average", getAverage).Methods("POST")

	r.HandleFunc("/{id}", getWebhook).Methods("GET")
	r.HandleFunc("/{id}", deleteWebhook).Methods("DELETE")
}
func getLatest(w http.ResponseWriter, r *http.Request) {
	var latest LatestRates
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&latest)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "incorrect body")
		return
	}

	var curr DataList
	err2 := database.C("currency").Find(nil).Sort("-_id").One(&curr)

	if err2 != nil {
		w.WriteHeader(404)
		fmt.Fprintf(w, "Internal error")
		return
	}
	rate := curr.As(latest.BaseCurrency).To(latest.TargetCurrency)
	fmt.Fprint(w, rate.Rate)
}
func getAverage(w http.ResponseWriter, r *http.Request) {
	var latest LatestRates
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&latest)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid data")
		return
	}

	var curr []DataList
	err2 := database.C("currency").Find(nil).Limit(7).Sort("-_id").All(&curr)
	if err2 != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal error")
		return
	}
	var avg float32
	avg = 0
	for i := range curr {
		item := curr[i]
		avg += item.From(latest.BaseCurrency).To(latest.TargetCurrency).Rate
	}
	fmt.Fprint(w, avg/7)
}
func evaluateTrigger(w http.ResponseWriter, req *http.Request) {
	var curr DataList
	err2 := database.C("currency").Find(nil).Sort("_id").One(&curr)

	if err2 != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal error")
		return
	}

	var results []PostWebhook
	dbErr := database.C("webhooks").Find(bson.M{}).All(&results)

	if dbErr != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal error")
		return
	} else {
		for i := range results {
			item := results[i]
			curr := curr.As(item.BaseCurrency).To(item.TargetCurrency)
			InvokeWebHook(curr, item)
		}
		fmt.Fprint(w, "Invoking webooks")

	}
}

func InvokeWebHook(curr Convertion, item PostWebhook) {
	inv := PostWebhookInvoked{
		BaseCurrency:    curr.From,
		TargetCurrency:  curr.To,
		CurrentRate:     curr.Rate,
		MinTriggerValue: item.MinTriggerValue,
		MaxTriggerValue: item.MaxTriggerValue,
	}
	data := new(bytes.Buffer)
	json.NewEncoder(data).Encode(inv)
	_, err := http.Post(item.WebhookURL, "application/json; charset=utf-8", data)
	if err != nil {
		fmt.Println("Error invoking webhook")
	}
}

func getWebhook(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	var webhook PostWebhook

	if !bson.IsObjectIdHex(vars["id"]) {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Internal error")
		return
	}
	id := bson.ObjectIdHex(vars["id"])

	err := database.C("webhooks").FindId(id).One(&webhook)
	if err != nil {
		w.WriteHeader(404)
		fmt.Fprintf(w, "Internal error")
	} else {

		output, jsonerr := json.MarshalIndent(webhook, "", "    ")
		if jsonerr != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Internal error")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		fmt.Fprintf(w, string(output))
	}
}

func deleteWebhook(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	if !bson.IsObjectIdHex(vars["id"]) {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Internal error")
		return
	}
	id := bson.ObjectIdHex(vars["id"])

	err := database.C("webhooks").RemoveId(id)
	if err != nil {
		w.WriteHeader(404)
		fmt.Fprintf(w, "Internal error")
	} else {
		w.WriteHeader(200)
		fmt.Fprintf(w, "")
	}
}

func getCronData() {
	resp, err := http.Get("https://api.fixer.io/latest")
	if err != nil {
		// fmt.Fprint(w, "Error")
		fmt.Println("Error fetching cron data")
		return
	}

	data1 := DataList{}
	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}
	jsonError := json.Unmarshal(body, &data1)

	if jsonError != nil {
		log.Fatal(jsonError)
	}
	latest := data1

	database.C("currency").Insert(data1)
	// latest := DataList{}
	var results []PostWebhook
	dbErr := database.C("webhooks").Find(bson.M{}).All(&results)
	if dbErr != nil {
		fmt.Println("Error fetching webhooks")
	} else {
		for i := range results {
			item := results[i]
			curr := latest.As(item.BaseCurrency).To(item.TargetCurrency)
			if curr.Rate > item.MaxTriggerValue || curr.Rate < item.MinTriggerValue {
				InvokeWebHook(curr, item)
			} else {
				fmt.Println("Results All: ", item.BaseCurrency)
			}
		}
	}
}
func handlerGet(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("https://api.fixer.io/latest")
	if err != nil {
		fmt.Fprint(w, "Error")
	}

	data1 := Data{}
	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}
	jsonError := json.Unmarshal(body, &data1)

	if jsonError != nil {
		log.Fatal(jsonError)
	}
	output, err := json.MarshalIndent(data1, "", "    ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintln(w, string(output))
}

func handlerPost(w http.ResponseWriter, r *http.Request) {
	var webhook PostWebhook
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&webhook)
	if err != nil {
		// ErrorWithJSON(w, "Incorrect body", http.StatusBadRequest)
		fmt.Println("Error decoding json")
		return
	}

	webhook.ID = bson.NewObjectId()
	collection := database.C("webhooks")
	collection.Insert(webhook)
	// .Insert(webhook)

	fmt.Fprintf(w, webhook.ID.Hex())
}

// From sets the base value of a currency
func (data DataList) From(name string) DataList {
	return data.As(name)
}

// To returns the value from a currency to another
func (data DataList) To(name string) Convertion {
	return Convertion{
		From:      data.Base,
		FromValue: 1.0,
		To:        name,
		ToValue:   data.Rates[name],
		Rate:      data.Rates[name],
	}
}

// As changes the base currency
func (data DataList) As(name string) DataList {
	if data.Base == name {
		return data
	}
	var baseCurrency float32
	if data.Base == "EUR" {
		baseCurrency = 1.0
	} else {
		baseCurrency = data.Rates[data.Base]
	}
	data.Rates[data.Base] = 1 * GetRates(data.Rates[name], baseCurrency)

	data.Base = name
	baseValue := data.Rates[name]
	for key, value := range data.Rates {
		data.Rates[key] = 1 * GetRates(baseValue, value)
	}
	delete(data.Rates, name)

	return data
}

// GetRates returns the currency rates
func GetRates(from float32, to float32) float32 {
	if from == to {
		return 1.0
	}
	return to * (1 / from)
}
