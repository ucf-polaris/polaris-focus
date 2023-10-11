package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

type EventLocation struct {
	BuildingLong    float64    `json:"BuildingLong"`
	BuildingLat     float64    `json:"BuildingLat"`
}
type Event struct {
	EventID     	string        `json:"EventID"`
	DateTime    	string        `json:"dateTime"`
	Description 	string        `json:"description"`
	Host        	string        `json:"host"`
	Location    	EventLocation `json:"location"`
	ListedLocation 	string 		  `json:"listedLocation,omitempty"`
	Image 			string 	      `json:"image,omitempty"`
	EndsOn 			string        `json:"endsOn,omitempty"`
	Name        	string        `json:"name"`
}
type Payload struct {
	Name     string `json:"name"`
	DateTime string `json:"dateTime"`
}
type Response struct {
	Tokens Tokens  `json:"tokens"`
}
type Tokens struct {
	Token        string `json:"token,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

// 1. use table and client as variable names
var table string
var client *dynamodb.Client

func init() {
	// 2. Use the helper function for dynamodb host
	client, table = Helpers.ConstructDynamoHost()

	// 3. error checking on env variables
	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}
}

// 4. only request in parameters, replace ctx to context.Background()
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 5. Add tokens
	token, refreshToken, err := Helpers.GetTokens(request)
	// 6. Proper error checking
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	var payload Payload
    err = json.Unmarshal([]byte(request.Body), &payload)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	name := payload.Name
	dateTime := payload.DateTime
	if name == "" || dateTime == "" {
		return Helpers.ResponseGeneration(fmt.Sprintf("Event name and dateTime missing"), http.StatusBadRequest)
	}

	eventObj, err := getEventByIndex(context.Background(), name, dateTime)
	if err != nil {
		return Helpers.ResponseGeneration(fmt.Sprintf("fetching Event data: %v", err), http.StatusBadRequest)
	}
	if len(eventObj) == 0 {
		return Helpers.ResponseGeneration(fmt.Sprintf("No event found with that name and dateTime"), http.StatusBadRequest)
	}

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"EventID": &types.AttributeValueMemberS{Value: eventObj[0].EventID},
		},
	}

	_, err = client.DeleteItem(context.Background(), input)
	if err != nil {
		if strings.Contains(err.Error(), "ConditionalCheckFailedException") {
			return Helpers.ResponseGeneration("Event not found in the table", http.StatusBadRequest)
		}
		return Helpers.ResponseGeneration(fmt.Sprintf("deleting item: %v", err), http.StatusInternalServerError)
	}

	// 7. Pack the tokens with the struct
	tokens := Tokens{
		Token: token,
		RefreshToken: refreshToken,
	}

	ret := Response{
		Tokens: tokens,
	}
	retJSON, err := json.Marshal(ret)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:		string(retJSON),
		Headers:	map[string]string{"content-type": "application/json"},
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