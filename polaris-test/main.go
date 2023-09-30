package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type EventLocation struct {
	BuildingLong float64 `json:"BuildingLong"`
	BuildingLat  float64 `json:"BuildingLat"`
}
type Event struct {
	EventID     string        `json:"EventID"`
	DateTime    string        `json:"dateTime"`
	Description string        `json:"description"`
	Host        string        `json:"host"`
	Location    EventLocation `json:"location"`
	Name        string        `json:"name"`
}
type Payload struct {
	Name     string `json:"name"`
	DateTime string `json:"dateTime"`
}

type Response struct {
	Events []Event `json:"events"`
	Tokens Tokens  `json:"tokens"`
}

type Tokens struct {
	Token        string `json:"token,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

// 1. Define as table and client
var table string
var client *dynamodb.Client

func init() {
	// 2. client, table set equal to Helper function
	client, table = Helpers.ConstructDynamoHost()

	// 3. Error checking on env variables
	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}
}

// 4. Only request in parameters and replace ctx with context.Background()
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	// 5. (if need to be protected) Add tokens
	token, refreshToken, err := Helpers.GetTokens(request)
	// 6. Proper error checking
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	// Create a payload struct
	var payload Payload
	// Unmarshal the request body into the payload struct and error check it
	err = json.Unmarshal([]byte(request.Body), &payload)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	// Extract the ID and use this to get the item from dynamo
	name := payload.Name
	dateTime := payload.DateTime

	// Fetch the Event in the form of a go struct from the database
	event, err := getEventByIndex(context.Background(), name, dateTime)

	// If an error came up, early exit and return the error
	if err != nil {
		return Helpers.ResponseGeneration(fmt.Sprintf("fetching Event data: %v", err), http.StatusBadRequest)
	}

	// If the Event didn't end up existing, return that information to the caller
	if event == nil {
		return Helpers.ResponseGeneration("Event not found in table", http.StatusBadRequest)
	}

	// 7. Pack the tokens with the struct
	tokens := Tokens{
		Token:        token,
		RefreshToken: refreshToken,
	}

	ret := Response{
		Events: event,
		Tokens: tokens,
	}

	// Convert the Event go struct to a json for return
	eventJSON, err := json.Marshal(ret)
	// If marshaling failed, early exit
	if err != nil {
		return Helpers.ResponseGeneration("Event not found in table", http.StatusBadRequest)
	}

	// Return the Event info in the form of a stringified JSON
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(eventJSON),
		Headers:    map[string]string{"content-type": "application/json"},
	}, nil
}

func getEventByIndex(ctx context.Context, name, dateTime string) ([]Event, error) {
	// Construct the get item input given the Event ID provided
	inp := &dynamodb.QueryInput{
		TableName:                aws.String(table),
		IndexName:                aws.String("name-dateTime-index"),
		KeyConditionExpression:   aws.String("#name = :name AND #dateTime = :dateTime"),
		ExpressionAttributeNames: map[string]string{"#name": "name", "#dateTime": "dateTime"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name":     &types.AttributeValueMemberS{Value: name},
			":dateTime": &types.AttributeValueMemberS{Value: dateTime},
		},
	}

	// Try to query dynamodb with this get item
	output, err := client.Query(ctx, inp)

	// Return the error if it fails
	if err != nil {
		return nil, err
	}

	// Return nil if the item didn't end up existing
	if output.Count == 0 {
		return nil, nil
	}

	// construct the go struct from dynamo's item
	event := []Event{}
	err = attributevalue.UnmarshalListOfMaps(output.Items, &event)
	if err != nil { // if this failed, early exit
		return nil, err
	}

	// yay!
	return event, nil
}

func main() {
	lambda.Start(handler)
}
