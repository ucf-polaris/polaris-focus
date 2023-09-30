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

	// extract the event id from the payload
	id := payload.EventID
	// if the ID is mising, early exit
	if id == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Event ID is missing",
		}, nil
	}

	// Now that we know the event exists, construct the delete input
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"EventID": &types.AttributeValueMemberS{Value: id},
		},
		ConditionExpression: aws.String("attribute_exists(EventID)"),
	}

	// Try to delete the user from the table and catch errors
	_, err = client.DeleteItem(ctx, input)
	if err != nil {
		log.Printf("%+v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Error when deleting event from table, event may not exist"),
		}, nil
	}

	// yay!
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Event deleted successfully",
	}, nil
}

func main() {
	lambda.Start(handler)
}