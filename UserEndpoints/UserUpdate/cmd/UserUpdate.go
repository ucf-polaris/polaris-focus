package main

import (
	"context"
	"log"
	"net/http"
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
	// create instance of user struct
	var usr User
	// unmarshal the raw input into a go struct type
	err := json.Unmarshal([]byte(request.Body), &usr)

	// if parsing failed then the input was invalid, let the caller know and return.
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Invalid input",
		}, nil
	}
	// debug
	log.Printf("Parsed User: %+v", usr)

	// Construct the required update input for dynamodb
	updateInput := &dynamodb.UpdateItemInput {
		// table name is a global variable
		TableName: &table,
		// Partitiion key for user table is UserID
		Key: map[string]types.AttributeValue{
			"UserID": &types.AttributeValueMemberS{
				Value: usr.UserID,
			},
		},
		// "SET" update expression to update the item in the table.
		UpdateExpression: aws.String("SET email = :email, password = :password, schedule = :schedule, username = :username, #N = :name"),
		ExpressionAttributeNames: map[string]string{"#N": "name",},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: usr.Email},
			":password": &types.AttributeValueMemberS{Value: usr.Password},
			":schedule": &types.AttributeValueMemberSS{Value: usr.Schedule},
			":username": &types.AttributeValueMemberS{Value: usr.Username},
			":name": &types.AttributeValueMemberS{Value: usr.Name},
		},
	}

	// Try to update the item, if it failed return and tell the user
	_, err = db.UpdateItem(ctx, updateInput)
	if err != nil {
		log.Printf("Error: %v", err)
		return events.APIGatewayProxyResponse {
			StatusCode: http.StatusInternalServerError,
			Body: "Error updating item",
		}, nil
	}

	// Great success!
    return events.APIGatewayProxyResponse{
        StatusCode: http.StatusOK,
        Body:       "User updated successfully!",
    }, nil
}

func main() {
	lambda.Start(handler)
}
