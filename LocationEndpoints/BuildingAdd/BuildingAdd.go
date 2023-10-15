package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
)

var table string
var client *dynamodb.Client

func init() {
	//create session for dynamodb
	client, table = Helpers.ConstructDynamoHost()

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}
}

func main() {
	lambda.Start(handler)
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------EXTRACT TOKEN FIELDS-----------------------------------------
	token, rfsTkn, err := Helpers.GetTokens(request)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}
	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	search := Helpers.UnpackRequest(request.Body)

	item, _, _, err := Helpers.ExtractFields(
		[]string{"BuildingLong", "BuildingLat", "BuildingDesc", "BuildingName"},
		search,
		false,
		false)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	optional_item, _, _, err := Helpers.ExtractFields(
		[]string{"BuildingEvents", "BuildingImage", "BuildingAltitude", "BuildingAbbreviation", "BuildingAllias", "BuildingAddress", "BuildingLocationType"},
		search,
		false,
		true)

	item = Helpers.MergeMaps(item, optional_item)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//format building events (if empty erase, else convert to string set)
	Helpers.ListToStringSet(
		[]string{":BuildingEvents", ":BuildingAllias", ":BuildingAbbreviation"},
		item,
		false,
	)
	//-----------------------------------------GET KEYS TO FILTER-----------------------------------------
	keys, _, _, err := Helpers.ExtractFields(
		[]string{"BuildingLong", "BuildingLat"},
		search,
		true,
		false)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}
	//-----------------------------------------PUT INTO DATABASE-----------------------------------------

	_, err = client.PutItem(context.Background(), &dynamodb.PutItemInput{
		ExpressionAttributeValues: keys,
		TableName:                 aws.String(table),
		Item:                      item,
		ConditionExpression:       aws.String("BuildingLong <> :BuildingLong AND BuildingLat <>  :BuildingLat"),
	})

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}
	//-----------------------------------------PACK RETURN VALUES-----------------------------------------
	ret := make(map[string]interface{})
	tokens := make(map[string]interface{})
	if token != "" {
		tokens["token"] = token
	}

	if rfsTkn != "" {
		tokens["refreshToken"] = rfsTkn
	}

	ret["tokens"] = tokens

	js, err := json.Marshal(ret)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
