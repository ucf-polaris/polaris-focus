package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Payload struct {
	BuildingLong float64 `json:"BuildingLong"`
	BuildingLat  float64 `json:"BuildingLat"`
}
type Response struct {
	Tokens   Tokens   `json:"tokens"`
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

	// extract long and lat from the payload
	blat := payload.BuildingLat
	blong := payload.BuildingLong
	// if the blong and blat were not found, the float64 becomes 0.0 0.0
	// these coordinates are not in the scope of ucf, so it isn't a problem to use
	if blat == 0.0 || blong == 0.0 {
		return Helpers.ResponseGeneration(fmt.Sprintf("Building long or lat missing"), http.StatusBadRequest)
	}

	// Construct delete input
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"BuildingLong": &types.AttributeValueMemberN{Value: strconv.FormatFloat(blong, 'f', -1, 64)},
			"BuildingLat":  &types.AttributeValueMemberN{Value: strconv.FormatFloat(blat, 'f', -1, 64)},
		},
		ConditionExpression: aws.String("attribute_exists(BuildingLong) AND attribute_exists(BuildingLat)"),
	}

	// delete building and error check it
	_, err = client.DeleteItem(context.Background(), input)
	if err != nil {
		return Helpers.ResponseGeneration(fmt.Sprintf("Error when deleting building: %+v", err), http.StatusInternalServerError)
	}

	tokens := Tokens{
		Token:        token,
		RefreshToken: refreshToken,
	}

	ret := Response{
		Tokens:   tokens,
	}
	retJSON, err := json.Marshal(ret)
	if err != nil {
		return Helpers.ResponseGeneration("when marshaling data", http.StatusBadRequest)
	}
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(retJSON),
		Headers:    map[string]string{"content-type": "application/json"},
	}, nil
}

func main() {
	lambda.Start(handler)
}
