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

type User struct {
	UserID           string   `json:"UserID"`
	Email            string   `json:"email"`
	Password         string   `json:"password"`
	Schedule         []string `json:"schedule"`
	Username         string   `json:"username"`
	Name             string   `json:"name"`
}

var table string
var db *dynamodb.Client

func init() {
	table = "Users"
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load config, %v", err)
	}
	db = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Extract user ID from request
	id, good := request.QueryStringParameters["UserID"]
	// If UserID wasn't present, return and ask for the user ID
	if !good {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:		"UserID is required",
		}, nil
	}

	// Fetch the user in the form of a go struct from the database
	usr, err := getUserByID(ctx, id)
	// If an error came up, early exit and return the error
	if err != nil {
		log.Printf("Error: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:		fmt.Sprintf("Error fetching user data: %v", err),
		}, err
	}

	// If the user didn't end up existing, return that information to the user
	if usr == nil {
		return events.APIGatewayProxyResponse {
			StatusCode: http.StatusBadRequest,
			Body: 		"User not found in table",
		}, nil
	}

	// Convert the user go struct to a json for return
	usrJSON, err := json.Marshal(usr)
	// If marshaling failed, early exit
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body: 		"Error converting user data to JSON",
		}, err
	}

	// Return the user info in the form of a stringified JSON
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body: 		string(usrJSON),
	}, nil
}

func getUserByID(ctx context.Context, userID string) (*User, error) {
	// Construct the get item input given the user ID provided
	inp := &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"UserID": &types.AttributeValueMemberS{Value: userID},
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

	// construct the go struct from dynamo's item
	usr := &User{}
	err = attributevalue.UnmarshalMap(output.Item, usr)
	if err != nil { // if this failed, early exit
		return nil, err
	}
	
	// yay!
	return usr, nil
}

func main() {
	lambda.Start(handler)
}