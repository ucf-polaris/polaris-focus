package main

import (
	"Helpers"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
)

var table string
var client *dynamodb.Client

func init() {
	table = os.Getenv("TABLE_NAME")

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}

	//create session for dynamodb
	cfg, _ := config.LoadDefaultConfig(context.Background())
	client = dynamodb.NewFromConfig(cfg)
}

func main() {
	lambda.Start(handler)
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------EXTRACT TOKEN FIELDS-----------------------------------------
	token, rfsTkn, err := Helpers.GetTokens(request)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}
	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	search := Helpers.UnpackRequest(request.Body)

	item, _, _, err := Helpers.ExtractFields(
		[]string{"BuildingLong", "BuildingLat", "BuildingDesc", "BuildingEvents", "BuildingName"},
		search,
		false,
		false)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}
	//-----------------------------------------GET KEYS TO FILTER-----------------------------------------
	keys, _, _, err := Helpers.ExtractFields(
		[]string{"BuildingLong", "BuildingLat"},
		search,
		true,
		false)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}
	//-----------------------------------------PUT INTO DATABASE-----------------------------------------

	_, err = client.PutItem(context.Background(), &dynamodb.PutItemInput{
		ExpressionAttributeValues: keys,
		TableName:                 aws.String(table),
		Item:                      item,
		ConditionExpression:       aws.String("BuildingLong <> :BuildingLong AND BuildingLat <>  :BuildingLat"),
	})

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}
	//-----------------------------------------PACK RETURN VALUES-----------------------------------------
	ret := make(map[string]interface{})
	if token != "" {
		ret["token"] = token
	}

	if rfsTkn != "" {
		ret["refreshToken"] = rfsTkn
	}

	js, err := json.Marshal(ret)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
