package main

import (
	"context"
	"log"
	"net/http"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/aws"
)

type EventLocation struct {
	BuildingLong    float64    `json:"BuildingLong"`
	BuildingLat     float64    `json:"BuildingLat"`
}
type Event struct {
	EventID         string          `json:"EventID"`
	DateTime        string          `json:"dateTime"`
	Description     string          `json:"description"`
	Host            string          `json:"host"`
	Location        EventLocation   `json:"location"`
	Name            string          `json:"name"`
}
type Payload struct {
	EventID			string			`json:"EventID"`
}

var table string
var db *dynamodb.Client

func init() {
	table = "Events"
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load config, %v", err)
	}
	db = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Create a payload struct
	var payload Payload
	// Unmarshal the request body into the payload struct and error check it
	err := json.Unmarshal([]byte(request.Body), &payload)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode:	http.StatusBadRequest,
			Body: 		"Invalid input format",
		}, nil
	}
	// Extract the ID and use this to get the item from dynamo
	id := payload.EventID

	// Fetch the Event in the form of a go struct from the database
	event, err := getEventByID(ctx, id)
	// If an error came up, early exit and return the error
	if err != nil {
		log.Printf("Error: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:		fmt.Sprintf("Error fetching Event data: %v", err),
		}, err
	}

	// If the Event didn't end up existing, return that information to the caller
	if event == nil {
		return events.APIGatewayProxyResponse {
			StatusCode: http.StatusBadRequest,
			Body: 		"Event not found in table",
		}, nil
	}

	// Convert the Event go struct to a json for return
	eventJSON, err := json.Marshal(event)
	// If marshaling failed, early exit
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body: 		"Error converting Event data to JSON",
		}, err
	}

	// Return the Event info in the form of a stringified JSON
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body: 		string(eventJSON),
	}, nil
}

func getEventByID(ctx context.Context, EventID string) (*Event, error) {
	// Construct the get item input given the Event ID provided
	inp := &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"EventID": &types.AttributeValueMemberS{Value: EventID},
		},
	}

	// Try to query dynamodb with this get item
	output, err := db.GetItem(ctx, inp)

	// Return the error if it fails
	if err != nil {
		return nil, err
	}

	// Return nil if the item didn't end up existing
	if output.Item == nil {
		return nil, nil
	}
	log.Printf("Dynamo return: %v", output.Item)
	// construct the go struct from dynamo's item
	event := &Event{}
	err = attributevalue.UnmarshalMap(output.Item, event)
	if err != nil { // if this failed, early exit
		return nil, err
	}
	
	// yay!
	return event, nil
}

func main() {
	lambda.Start(handler)
}