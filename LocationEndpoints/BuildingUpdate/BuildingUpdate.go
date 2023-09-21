package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"polaris-api/pkg/Helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	search := Helpers.UnpackRequest(request.Body)

	items, queryString, mapQuery, err := Helpers.ExtractFields(
		[]string{"BuildingDesc", "BuildingEvents", "BuildingName"},
		search,
		true,
		true)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}
	//-----------------------------------------GET KEYS TO FILTER-----------------------------------------
	key, _, _, err := Helpers.ExtractFields(
		[]string{"BuildingLong", "BuildingLat"},
		search,
		false,
		false)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//put keys in ExpressionAttributeValues for ConditionExpression
	items[":BuildingLong"] = key["BuildingLong"]
	items[":BuildingLat"] = key["BuildingLat"]
	//-----------------------------------------PUT INTO DATABASE-----------------------------------------
	updateInput := &dynamodb.UpdateItemInput{
		// table name is a global variable
		TableName: &table,
		// Partitiion key for user table is EventID
		Key: key,
		// "SET" update expression to update the item in the table.
		UpdateExpression:          aws.String(queryString),
		ExpressionAttributeNames:  mapQuery,
		ExpressionAttributeValues: items,
		ReturnValues:              types.ReturnValueUpdatedNew,
		//don't make new record if key doesn't exist (could take this out and make a new add?)
		ConditionExpression: aws.String("BuildingLong = :BuildingLong AND BuildingLat = :BuildingLat"),
	}

	retValues, err := client.UpdateItem(context.Background(), updateInput)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------PACK RETURN VALUES-----------------------------------------
	ret := make(map[string]interface{})
	attributevalue.UnmarshalMap(retValues.Attributes, &ret)
	if token != "" {
		ret["token"] = token
	}

	if rfsTkn != "" {
		ret["refreshToken"] = rfsTkn
	}

	js, err := json.Marshal(ret)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
