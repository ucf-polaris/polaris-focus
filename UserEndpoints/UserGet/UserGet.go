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

type User struct {
	UserID           string   `json:"UserID,omitempty"`
	Email            string   `json:"email"`
	Password         string   `json:"password,omitempty"`
	Schedule         []string `json:"schedule"`
	Favorite		 []string `json:"favorite"`
	Visited			 []string `json:"visited"`
	Username         string   `json:"username"`
	Name             string   `json:"name"`
}
type Payload struct {
	Email			string		`json:"email"`
}
type Response struct {
	Users []User `json:"users"`
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

	var payload Payload
    err = json.Unmarshal([]byte(request.Body), &payload)
    if err != nil {
        return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
    }
	email := payload.Email

	// Fetch the user in the form of a go struct from the database
	usr, err := getUserByEmail(context.Background(), email)
	// If an error came up, early exit and return the error
	if err != nil {
		return Helpers.ResponseGeneration(fmt.Sprintf("fetching user data: %v", err), http.StatusBadRequest)
	}

	// If the user didn't end up existing, return that information to the user
	if usr == nil {
		return Helpers.ResponseGeneration("User not found in table", http.StatusBadRequest)
	}

	tokens := Tokens{
		Token:			token,
		RefreshToken: 	refreshToken,
	}

	ret := Response{
		Users: usr,
		Tokens: tokens,
	}

	// Convert the user go struct to a json for return
	usrJSON, err := json.Marshal(ret)
	// If marshaling failed, early exit
	if err != nil {
		return Helpers.ResponseGeneration("when marshaling data", http.StatusBadRequest)
	}

	// Return the user info in the form of a stringified JSON
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body: 		string(usrJSON),
		Headers:    map[string]string{"content-type": "application/json"},
	}, nil
}

func getUserByEmail(ctx context.Context, email string) ([]User, error) {
	// Construct the get item input given the Event ID provided
	inp := &dynamodb.QueryInput{
		TableName:                aws.String(table),
		IndexName:                aws.String("email-index"),
		KeyConditionExpression:   aws.String("email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email":     &types.AttributeValueMemberS{Value: email},
		},
		ProjectionExpression: aws.String("UserID, email, schedule, username, #name, visited, favorite"),
		ExpressionAttributeNames: map[string]string{"#name": "name"},
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
	usr := []User{}
	err = attributevalue.UnmarshalListOfMaps(output.Items, &usr)
	if err != nil { // if this failed, early exit
		return nil, err
	}
	
	// yay!
	return usr, nil
}

func main() {
	lambda.Start(handler)
}