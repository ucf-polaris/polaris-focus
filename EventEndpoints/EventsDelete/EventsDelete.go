package main

import (
	"context"
	"encoding/json"
	"net/http"
	"fmt"
	"log"
	"polaris-api/pkg/Helpers"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
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

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"name": &types.AttributeValueMemberS{Value: name},
			"dateTime": &types.AttributeValueMemberS{Value: dateTime},
		},
		ConditionExpression: aws.String("attribute_exists(name) AND attribute_exists(dateTime)"),
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

func main() {
	lambda.Start(handler)
}