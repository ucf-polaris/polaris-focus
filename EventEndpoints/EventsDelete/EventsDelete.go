package main

import (
	"context"
	"encoding/json"
	"net/http"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
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
	EventID         string          `json:"EventID"`
	DateTime        string          `json:"dateTime"`
	Description     string          `json:"description"`
	Host            string          `json:"host"`
	Location        EventLocation   `json:"location"`
	Name            string          `json:"name"`
}
type Payload struct {
	EventID		string		`json:"EventID"`
}

var table string
var client *dynamodb.Client

func init() {
	table = "Events"
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load config, %v", err)
	}
	client = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// iniitalize the payload structure
	var payload Payload
	// unmarshal the input and error check if something went wrong
    err := json.Unmarshal([]byte(request.Body), &payload)
    if err != nil {
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusBadRequest,
            Body:       "Invalid input format",
        }, nil
    }

	// extract event id from the payload
	id := payload.EventID
	// if the ID was missing, early exit
	if id == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "Event ID missing",
		}, nil
	}

	// First check if the event exists or not by getting it
	// We'll also use this to get its location to remove from there
	inp := &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"EventID": &types.AttributeValueMemberS{Value: id},
		},
	}

	// store the result and error of trying to get the event
	res, err := client.GetItem(ctx, inp)
	// failed to get the event
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Error when checking if event exists",
		}, nil
	}
	// early exit if the event doesn't exist
	if res.Item == nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusNotFound,
			Body:       "Event not found",
		}, nil
	}

	// put the event into the go struct by unmarshaling res.Item
	event := &Event{}
	err = attributevalue.UnmarshalMap(res.Item, event)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:		"Error unmarshaling event after getting it",
		}, nil
	}
	// Delete the entry in Building.BuildingEvents[] list that pertains to this event (it's a set of strings)
	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String("Buildings"),
		Key: map[string]types.AttributeValue{
			"BuildingLong": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", event.Location.BuildingLong)},
			"BuildingLat": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", event.Location.BuildingLat)},
		},
		UpdateExpression: aws.String("DELETE BuildingEvents :evtId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":evtId": &types.AttributeValueMemberSS{Value: []string{id}},
		},
	}

	// Update the building that has this event and remove the event id
	_, buildingErr := client.UpdateItem(ctx, updateInput)
	if buildingErr != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:		fmt.Sprintf("Failed to remove event from associated building: %+v", buildingErr),
		}, nil
	}

	// Now that we know the event exists and it has been removed from its building, get ready to delete it
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"EventID": &types.AttributeValueMemberS{Value: id},
		},
	}

	// delete event and error check it
	_, err = client.DeleteItem(ctx, input)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Error deleting item %+v", err),
		}, nil
	}

	// Finally, return that the item was successfully deleted as expected.
	// yay!
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Event deleted successfully and removed from its building",
	}, nil
}

func main() {
	lambda.Start(handler)
}