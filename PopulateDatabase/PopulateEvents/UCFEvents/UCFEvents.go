package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"polaris-api/pkg/Helpers"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// #region DEFINITIONS
var table string
var client *dynamodb.Client

type PageConfig struct {
	TotalLimit int `json:"abs_limit"`
}

type Event struct {
	EventID        string        `json:"id"`
	Host           string        `json:"contact_name"`
	Description    string        `json:"description"`
	Name           string        `json:"title"`
	ListedLocation string        `json:"location"`
	Location       LocationIndex `json:"loc,omitempty"`
	DateTime       string        `json:"starts"`
	EndsOn         string        `json:"ends"`
	Image          string        `json:"imagePath,omitempty"`
	TimeToLive     int64         `json:"timeTilExpire"`
	QueryLocation  string        `json:"locationQueryID"`
}

type LocationIndex struct {
	BuildingLong float64 `json:"BuildingLong"`
	BuildingLat  float64 `json:"BuildingLat"`
}

//#endregion DEFINITIONS

//#region HELPER_FUNCTIONS

func init() {
	//dynamo db
	client, table = Helpers.ConstructRealDynamoHost()
}

func renameFields(naming_map map[string]string, m map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for original, new := range naming_map {
		if _, ok := m[original]; ok {
			ret[new] = m[original]
			//delete(m, original)
		}
	}
	return ret
}

func convertToMap(e Event) map[string]interface{} {
	m2 := map[string]interface{}{}
	js, _ := json.Marshal(e)
	json.Unmarshal(js, &m2)
	return m2
}

// CSVToMap takes a reader and returns an array of dictionaries, using the header row as the keys
func CSVToMap(reader io.Reader) []map[string]string {
	r := csv.NewReader(reader)
	rows := []map[string]string{}
	var header []string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if header == nil {
			header = record
		} else {
			dict := map[string]string{}
			for i := range header {
				dict[header[i]] = record[i]
			}
			rows = append(rows, dict)
		}
	}
	return rows
}

// #endregion HELPER_FUNCTIONS

// #region LOCATION_EVAL_FUNCTIONS

func checkFullName(locations []map[string]string, e Event) string {
	for _, ele := range locations {
		if strings.ToLower(ele["Name"]) == strings.ToLower(e.ListedLocation) {
			//log.Println("1. found ", e.ListedLocation, "at", ele["Name"])
			return ele["Latitude, Longitude"]
		}
	}
	return ""
}

func checkPartialName(locations []map[string]string, e Event) string {
	for _, ele := range locations {
		non_spec := strings.ToLower(replaceSpecials(e.ListedLocation))
		if matchWord(strings.ToLower(ele["Name"]), non_spec) {
			//log.Println("2. found ", non_spec, "at", ele["Name"])
			return ele["Latitude, Longitude"]
		}
	}
	return ""
}

func checkPartialAbbre(locations []map[string]string, e Event) string {
	for _, ele := range locations {
		//error checking
		if ele["Abbreviation"] == "" {
			continue
		}
		//split the comma seperated string
		abbs := strings.Split(ele["Abbreviation"], ",")
		non_spec := strings.ToLower(replaceSpecials(e.ListedLocation))

		for _, abb := range abbs {
			//search for abbreviation (regularly)
			if matchWord(strings.ToLower(abb), non_spec) {
				//log.Println("3. found ", non_spec, "at", abb)
				return ele["Latitude, Longitude"]
			}

			//search for abbreviation without numbers
			if matchWord(strings.ToLower(abb), replaceNumbers(non_spec)) {
				//log.Println("4. found ", e.ListedLocation, "at", ele["Name"])
				return ele["Latitude, Longitude"]
			}
		}
	}

	return ""
}

func checkPartialAlias(locations []map[string]string, e Event) string {
	for _, ele := range locations {
		//error checking
		if ele["Alias"] == "" {
			continue
		}
		//split the comma seperated string
		aliases := strings.Split(ele["Alias"], ",")
		non_spec := strings.ToLower(replaceSpecials(e.ListedLocation))

		for _, alias := range aliases {
			//search for aliases
			if matchWord(strings.ToLower(alias), non_spec) {
				//log.Println("5. found ", e.ListedLocation, "at", alias, ele["Name"])
				return ele["Latitude, Longitude"]
			}
		}
	}

	return ""
}

func matchWord(w, s string) bool {
	r, _ := regexp.Compile(`\b` + w + `\b`)
	//s is the bigger one
	return r.MatchString(s)
}

func replaceNumbers(s string) string {
	re := regexp.MustCompile(`[0-9]+`)
	return re.ReplaceAllString(s, " ")
}

func replaceSpecials(s string) string {
	re := regexp.MustCompile(`[^A-Za-z0-9 ]+`)
	return re.ReplaceAllString(s, " ")
}

// #endregion LOCATION_EVAL_FUNCTIONS

// #region UTILITY_FUNCTIONS
func makeRequest(page string) ([]Event, error) {
	//construct url and make request
	url_string := "https://events.ucf.edu/upcoming/feed.json?page=" + page

	resp, err := http.Get(url_string)
	if err != nil {
		return nil, err
	}

	//read in data
	body, _ := ioutil.ReadAll(resp.Body)
	output := []Event{}
	json.Unmarshal(body, &output)

	return output, nil
}

func massRename(e []Event) []map[string]interface{} {
	//take everything within e ([]Event) and convert them to map interface then rename them to fit with request
	ret := []map[string]interface{}{}
	for _, ele := range e {
		m2 := convertToMap(ele)
		m2 = renameFields(
			map[string]string{
				"location":        "listedLocation",
				"loc":             "location",
				"imagePath":       "image",
				"starts":          "dateTime",
				"contact_name":    "host",
				"id":              "EventID",
				"ends":            "endsOn",
				"title":           "name",
				"description":     "description",
				"locationQueryID": "locationQueryID",
				"timeTilExpire":   "timeTilExpire",
			},
			m2,
		)
		//log.Println(m2)
		ret = append(ret, m2)
	}

	return ret
}

func handleLocations(e []Event) ([]Event, error) {
	//import csv file as dict
	csvFile, err := os.Open("Polaris-Locations.csv")
	if err != nil {
		return e, err
	}

	defer csvFile.Close()

	records := CSVToMap(csvFile)

	//iterate through events
	for i, event := range e {
		ret := ""

		//check until ret isn't empty
		// 1. full Name
		// 2. partial Name
		// 3. partial Abbreviation
		// 4. partial Alias
		ret = checkFullName(records, event)
		if ret == "" {
			ret = checkPartialName(records, event)
		}
		if ret == "" {
			ret = checkPartialAbbre(records, event)
		}
		if ret == "" {
			ret = checkPartialAlias(records, event)
		}

		if ret == "" {
			ret = "0, 0"
		}

		//parse location string
		mystr := strings.Replace(ret, " ", "", -1)
		array := strings.Split(mystr, ",")

		//convert to float
		long, _ := strconv.ParseFloat(array[1], 64)
		lat, _ := strconv.ParseFloat(array[0], 64)

		//pack into locations
		e[i].Location = LocationIndex{
			BuildingLong: long,
			BuildingLat:  lat,
		}

		//make locationID
		e[i].QueryLocation = array[1] + " " + array[0]

		//make ttl
		e[i], err = makeTTL(e[i], 0)
		if err != nil {
			return e, err
		}
	}

	return e, nil
}

func makeTTL(event Event, expire int) (Event, error) {
	date := event.EndsOn
	thetime, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return event, err
	}

	var timeVal int64
	if expire <= 0 {
		timeVal = thetime.UTC().Add(time.Hour * 24).Unix()
	} else {
		timeVal = thetime.UTC().Add(time.Hour * time.Duration(expire)).Unix()
	}

	//make sure dates aren't older than the current day (or by 5 years)
	event.TimeToLive = timeVal

	return event, nil
}

func correctTimeFormat(e []Event, format string) ([]Event, error) {
	ret := e
	for i, ele := range e {
		//correct datetime
		t, err := time.Parse(format, ele.DateTime)
		if err != nil {
			return e, err
		}
		ret[i].DateTime = t.Format(time.RFC3339)

		t, err = time.Parse(format, ele.EndsOn)
		if err != nil {
			return e, err
		}
		ret[i].EndsOn = t.Format(time.RFC3339)
	}

	return ret, nil
}

func compareTime(e []Event, days int) bool {
	t, _ := time.Parse(time.RFC1123Z, e[len(e)-1].EndsOn)
	comp1 := t.UTC().Unix()
	comp2 := time.Now().UTC().Add(time.Hour * 24 * time.Duration(days)).Unix()

	return comp1 >= comp2
}

// #endregion UTILITY_FUNCTIONS

// #region DATABASE_FUNCTIONS
func constructRequest(events []map[string]interface{}, limit int) []*dynamodb.BatchWriteItemInput {
	//limit the 'limit' parameter
	if limit <= 0 || limit > 20 {
		limit = 20
	}

	requests := []*dynamodb.BatchWriteItemInput{}
	request := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			table: {},
		},
	}

	//iterate through all events
	for _, event := range events {
		av, _ := attributevalue.MarshalMap(event)

		//build request
		construct := types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: av,
			},
		}

		request.RequestItems[table] = append(request.RequestItems[table], construct)

		//check if this request exceeds limit
		if len(request.RequestItems[table]) >= limit {
			//deep copy struct
			foo := *request

			requests = append(requests, &foo)

			//reset request
			request = &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					table: {},
				},
			}
		}
	} //End for loop

	if len(request.RequestItems[table]) != 0 {
		foo := *request
		requests = append(requests, &foo)
	}
	return requests
}

func ExecutePut(requests []*dynamodb.BatchWriteItemInput) error {

	for _, ele := range requests {
		_, err := client.BatchWriteItem(context.TODO(), ele)

		if err != nil {
			log.Println(err)
		}
	}

	return nil
}

// #endregion DATABASE_FUNCTIONS

// #region MAIN_FUNCTIONS
func main() {
	lambda.Start(handler)
}

func handler(config PageConfig) (map[string]interface{}, error) {
	//-----------------------------------------DEFINE VARIABLES-----------------------------------------
	payload := []map[string]interface{}{}

	//how many days in advance should the program parse
	str_days := os.Getenv("ADVANCE_PAGES")
	if str_days == "" {
		str_days = "3"
	}

	pages, err := strconv.Atoi(str_days)
	if err != nil {
		return nil, err
	}

	if pages <= 0 {
		pages = 3
	}

	//-----------------------------------------CONSTRUCT PAYLOAD-----------------------------------------
	var page int = 1
	var processed int = 0
	//get the Events array ready
	for {
		//make request in batches
		result, err := makeRequest(strconv.Itoa(page))
		if err != nil {
			return nil, err
		}

		if len(result) == 0 {
			break
		}

		processed += len(result)
		log.Println("Processed:", processed)

		//correct formatting of times
		result, err = correctTimeFormat(result, time.RFC1123Z)
		if err != nil {
			return nil, err
		}

		//assign locations to result here
		result, err = handleLocations(result)
		if err != nil {
			return nil, err
		}

		payload = append(payload, massRename(result)...)

		//if met amount of days, break out of loop
		if pages <= page {
			break
		}

		page += 1
	}

	//-----------------------------------------ADD TO DATABASE-----------------------------------------
	requests := constructRequest(payload, 20)
	err = ExecutePut(requests)
	if err != nil {
		return nil, err
	}

	counter, err := Helpers.IncrementCounterTable(client, "EventParseAmount", "Counters")
	if err != nil {
		return nil, err
	}

	//-----------------------------------------SETUP RETURN-----------------------------------------
	ret := map[string]interface{}{
		"total":     len(payload),
		"processed": processed,
		"counter":   counter,
	}

	return ret, nil
}

//#endregion MAIN_FUNCTIONS
