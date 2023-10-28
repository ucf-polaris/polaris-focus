package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"polaris-api/pkg/DatabaseFuncs"
	"polaris-api/pkg/Helpers"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var location_table string
var event_table string
var client *dynamodb.Client

type Location struct {
	Long           float64  `json:"BuildingLong"`
	Lat            float64  `json:"BuildingLat"`
	BuildingEvents []string `json:"BuildingEvents"`
}

type Event struct {
	EventID string `json:"EventID"`
}

func getEventCount(l []Location) int {
	total := 0
	for _, location := range l {
		total += len(location.BuildingEvents)
	}

	return total
}

func eventsToStruct(m []map[string]interface{}) ([]Event, error) {
	//get m into a better to use enumerable
	iterate := []Event{}

	js, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(js, &iterate)
	if err != nil {
		return nil, err
	}

	return iterate, nil
}

func locationToStruct(m []map[string]interface{}) ([]Location, error) {
	//get m into a better to use enumerable
	iterate := []Location{}

	js, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(js, &iterate)
	if err != nil {
		return nil, err
	}

	return iterate, nil
}

func addToLocationMap(m map[string][]string, key, value string) {
	if _, ok := m[key]; !ok {
		m[key] = []string{}
	}

	m[key] = append(m[key], value)
}

func createEventMap(e []Event) map[string]bool {
	ret := make(map[string]bool)

	//iterate over locations to create map of buildings
	for _, event := range e {
		ret[event.EventID] = true
	}

	return ret
}

func init() {
	//dynamo db
	client, location_table = Helpers.ConstructRealDynamoHost()
	event_table = os.Getenv("EVENT_TABLE")

	location_table = "Buildings"
	event_table = "Events"
}

func compareEventsAndLocations(l []Location, e map[string]bool) (int, int, map[string][]string) {
	in_locations := 0
	not_in_locations := 0
	ret := map[string][]string{}

	for _, location := range l {
		for _, id := range location.BuildingEvents {
			//see if BuildingEvents has a valid event in Events
			if _, ok := e[id]; ok {
				in_locations += 1
			} else {
				not_in_locations += 1
				key := strconv.FormatFloat(location.Long, 'f', -1, 64) + " " + strconv.FormatFloat(location.Lat, 'f', -1, 64)
				addToLocationMap(ret, key, id)
			}
		}
	}

	return in_locations, not_in_locations, ret
}

func removeFromUpdate(m map[string][]string) (int, error) {
	total := 0
	for location, events := range m {
		arr := strings.Split(location, " ")

		long, _ := strconv.ParseFloat(arr[0], 64)
		lat, _ := strconv.ParseFloat(arr[1], 64)
		keys := map[string]interface{}{
			"BuildingLong": long,
			"BuildingLat":  lat,
		}

		items := map[string]interface{}{
			"events": events,
		}

		resp, err := DatabaseFuncs.UpdateDatabase("", "DELETE BuildingEvents :events", location_table, client, keys, items,
			[]string{
				":events",
			})
		if err != nil {
			return 0, err
		}

		log.Println(resp)
		total += 1
	}

	log.Println("Total Processed:", total)
	return total, nil
}

func main() {
	//lambda.Start(handler)
	resp, err := handler()

	if err != nil {
		log.Println(err)
	} else {
		log.Println(resp)
	}
}

func handler() (events.APIGatewayProxyResponse, error) {
	//get all locations
	locations, err := DatabaseFuncs.ScanDatabase(location_table, client)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//get all events
	evnts, err := DatabaseFuncs.ScanDatabase(event_table, client)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//get locations and events into structs for easier processing
	s_locations, err := locationToStruct(locations)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	s_events, err := eventsToStruct(evnts)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//create map for updating BuildingEvents database
	//ml := createLocationMap(s_locations)

	//create map for locating events
	me := createEventMap(s_events)

	in_data, not_in_data, m := compareEventsAndLocations(s_locations, me)
	removed, err := removeFromUpdate(m)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//construct return
	ret := map[string]interface{}{
		"events_in_locations": getEventCount(s_locations),
		"event_total":         len(evnts),
		"in_database":         in_data,
		"not_in_database":     not_in_data,
		"removed":             removed,
	}

	js, _ := json.Marshal(ret)

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
