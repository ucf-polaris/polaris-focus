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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Payload struct {
	UserID string `json:"UserID"`
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
	// iniitalize the  payload structure
	var payload Payload
	// unmarshal the input and error check if something went wrong
	err = json.Unmarshal([]byte(request.Body), &payload)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	// extract the user id from the payload
	id := payload.UserID
	// if the ID is mising, early exit
	if id == "" {
		return Helpers.ResponseGeneration(fmt.Sprintf("UserID missing"), http.StatusBadRequest)
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
	_, err = client.DeleteItem(context.Background(), input)
	if err != nil {
		if strings.Contains(err.Error(), "ConditionalCheckFailedException") {
			return Helpers.ResponseGeneration("User not found in the table", http.StatusBadRequest)
		}
		return Helpers.ResponseGeneration(fmt.Sprintf("Issue when deleting user from table: %v", err), http.StatusInternalServerError)
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

func main() {
	lambda.Start(handler)
}
