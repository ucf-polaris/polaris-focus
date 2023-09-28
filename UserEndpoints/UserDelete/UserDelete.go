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
	"github.com/aws/aws-sdk-go-v2/aws"
)

type Payload struct {
	UserID		string		`json:"UserID"`
}

var table string
var client *dynamodb.Client

func init() {
	table = "Users"
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load config, %v", err)
	}
	client = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// iniitalize the  payload structure
	var payload Payload
	// unmarshal the input and error check if something went wrong
    err := json.Unmarshal([]byte(request.Body), &payload)
    if err != nil {
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusBadRequest,
            Body:       "Invalid input format",
        }, nil
    }

	// extract the user id from the payload
	id := payload.UserID
	// if the ID is mising, early exit
	if id == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body: "User ID is missing",
		}, nil
	}

	// Now that we know the user exists, construct the delete input
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"UserID": &types.AttributeValueMemberS{Value: id},
		},
		ConditionExpression: aws.String("attribute_exists(UserID)"),
	}

	// Try to delete the user from the table and catch errors
	_, err = client.DeleteItem(ctx, input)
	if err != nil {
		log.Printf("%+v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:		fmt.Sprintf("Error when deleting user from table, user may not exist"),
		}, nil
	}

	// yay!
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:		"User deleted successfully",
	}, nil
}

func main() {
	lambda.Start(handler)
}