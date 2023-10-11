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
	PerPage    int `json:"per_page"`
	TotalLimit int `json:"abs_limit"`
}

type Event struct {
	EventID        string        `json:"id"`
	Host           string        `json:"organizationName"`
	Description    string        `json:"description"`
	Name           string        `json:"name"`
	ListedLocation string        `json:"location"`
	Location       LocationIndex `json:"loc,omitempty"`
	DateTime       string        `json:"startsOn"`
	EndsOn         string        `json:"endsOn"`
	Image          string        `json:"imagePath"`
	TimeToLive     int64         `json:"timeTilExpire"`
	QueryLocation  string        `json:"locationQueryID"`
}

type LocationIndex struct {
	BuildingLong float64 `json:"BuildingLong"`
	BuildingLat  float64 `json:"BuildingLat"`
}

type ReturnType struct {
	Value []Event `json:"value"`
	Count int     `json:"@odata.count"`
}

//#endregion DEFINITIONS

//#region HELPER_FUNCTIONS

func init() {
	//dynamo db
	client, table = Helpers.ConstructRealDynamoHost()
}

func getRelativeDate(days int) string {
	return time.Now().UTC().Add(time.Hour * 24 * time.Duration(days)).Format("2006-01-02")
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
			return ele["Latitude, Longitude"]
		}
	}
	return ""
}

func checkPartialName(locations []map[string]string, e Event) string {
	for _, ele := range locations {
		non_spec := strings.ToLower(replaceSpecials(e.ListedLocation))
		if matchWord(strings.ToLower(ele["Name"]), non_spec) {
			return ele["Latitude, Longitude"]
		}
	}
	return ""
}

func checkPartialAbbre(locations []map[string]string, e Event) string {
	for _, ele := range locations {
		//split the comma seperated string
		abbs := strings.Split(ele["Abbreviation"], ",")
		non_spec := strings.ToLower(replaceSpecials(e.ListedLocation))

		for _, abb := range abbs {
			//search for abbreviation (regularly)
			if matchWord(strings.ToLower(abb), non_spec) {
				return ele["Latitude, Longitude"]
			}

			//search for abbreviation without numbers
			if matchWord(strings.ToLower(abb), replaceNumbers(non_spec)) {
				return ele["Latitude, Longitude"]
			}
		}
	}

	return ""
}

func checkPartialAlias(locations []map[string]string, e Event) string {
	for _, ele := range locations {
		//split the comma seperated string
		aliases := strings.Split(ele["Alias"], ",")
		non_spec := strings.ToLower(replaceSpecials(e.ListedLocation))

		for _, alias := range aliases {
			//search for aliases
			if matchWord(strings.ToLower(alias), non_spec) {
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
func makeRequest(limit, processed, dateFuture, dateNow string) (ReturnType, error) {
	//construct url and make request
	url_string := "https://knightconnect.campuslabs.com/engage/api/discovery/event/search?startsAfter=" + dateNow + "&startsBefore=" + dateFuture + "&orderByField=endsOn&orderByDirection=ascending&status=Approved&take=" + limit + "&skip=" + processed

	resp, err := http.Get(url_string)
	if err != nil {
		return ReturnType{}, err
	}

	//read in data
	body, _ := ioutil.ReadAll(resp.Body)
	output := ReturnType{}
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
				"location":         "listedLocation",
				"loc":              "location",
				"imagePath":        "image",
				"startsOn":         "dateTime",
				"organizationName": "host",
				"id":               "EventID",
				"endsOn":           "endsOn",
				"name":             "name",
				"description":      "description",
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

func ExecutePut(requests []*dynamodb.BatchWriteItemInput) (int, error) {
	unprocessed := 0

	for _, ele := range requests {
		out, err := client.BatchWriteItem(context.TODO(), ele)

		if err != nil {
			log.Println(err)
		}

		unprocessed += len(out.UnprocessedItems[table])
	}

	return unprocessed, nil
}

// #endregion DATABASE_FUNCTIONS

// #region MAIN_FUNCTIONS
func main() {
	lambda.Start(handler)
}

func handler(config PageConfig) (map[string]interface{}, error) {
	//-----------------------------------------CONFIGURE PAGES-----------------------------------------
	limit := 50
	processed := 0
	if config.PerPage > 0 {
		limit = config.PerPage
	}
	//-----------------------------------------DEFINE VARIABLES-----------------------------------------
	payload := []map[string]interface{}{}

	//get dates

	//how many days in advance should the program parse
	str_days := os.Getenv("ADVANCE_DAYS")
	days, err := strconv.Atoi(str_days)
	if err != nil {
		return nil, err
	}
	if days < 0 {
		days = 5
	}
	date := getRelativeDate(0)
	inWeek := getRelativeDate(days)

	//get sample count
	sample, err := makeRequest("0", "0", inWeek, date)
	if err != nil {
		return nil, err
	}

	total := sample.Count
	if config.TotalLimit > 0 {
		total = config.TotalLimit
	}

	if total < limit {
		limit = total
	}

	//-----------------------------------------CONSTRUCT PAYLOAD-----------------------------------------
	//get the Events array ready
	for total > processed {
		//make request in batches
		result, err := makeRequest(strconv.Itoa(limit), strconv.Itoa(processed), inWeek, date)
		if err != nil {
			return nil, err
		}

		if result.Count == 0 {
			break
		}
		processed += len(result.Value)
		log.Println("Processed:", processed)

		//assign locations to result.Value here
		result.Value, err = handleLocations(result.Value)

		if err != nil {
			return nil, err
		}

		payload = append(payload, massRename(result.Value)...)
	}

	//-----------------------------------------ADD TO DATABASE-----------------------------------------
	requests := constructRequest(payload, 20)
	unprocessed, err := ExecutePut(requests)
	if err != nil {
		return nil, err
	}

	//-----------------------------------------SETUP RETURN-----------------------------------------
	ret := map[string]interface{}{
		"unprocessed": unprocessed,
		"total":       total,
	}

	return ret, nil
}

//#endregion MAIN_FUNCTIONS
