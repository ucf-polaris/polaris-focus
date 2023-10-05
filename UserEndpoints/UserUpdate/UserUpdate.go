package main

import (
	"context"
	"log"
	"net/http"
	"encoding/json"
	"polaris-api/pkg/Helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
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
type Response struct {
	Tokens Tokens  `json:"tokens"`
}

type Tokens struct {
	Token        string `json:"token,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
}
var table string
var client *dynamodb.Client

func init() {
	client, table = Helpers.ConstructDynamoHost()

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	token, refreshToken, err := Helpers.GetTokens(request)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}
	// create instance of user struct
	var usr User
	// unmarshal the raw input into a go struct type
	err = json.Unmarshal([]byte(request.Body), &usr)

	// if parsing failed then the input was invalid, let the caller know and return.
    if err != nil {
        return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
    }

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
	_, err = client.UpdateItem(context.Background(), updateInput)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	tokens := Tokens{
		Token:			token,
		RefreshToken:	refreshToken,
	}
	ret := Response{
		Tokens: tokens,
	}
	retJSON, err := json.Marshal(ret)
	if err != nil {
		return Helpers.ResponseGeneration("when marshaling data", http.StatusBadRequest)
	}
	// Great success!
    return events.APIGatewayProxyResponse{
        StatusCode: http.StatusOK,
        Body:       string(retJSON),
		Headers:	map[string]string{"content-type": "application/json"},
    }, nil
}

func main() {
	lambda.Start(handler)
}
