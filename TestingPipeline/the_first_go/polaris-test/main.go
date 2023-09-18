package main

import (
	"Helpers"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/uuid"
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

	item, _, _, errs := Helpers.ExtractFields(
		[]string{"name", "host", "description", "dateTime", "location"},
		search,
		false,
		false)

	if errs != nil {
		return Helpers.ResponseGeneration(errs.Error(), http.StatusOK)
	}

	uuid_new := uuid.Must(uuid.NewRandom()).String()
	//allows unit testing to be consistent
	if Helpers.IsLambdaLocal() {
		uuid_new = "0"
	}
	item["EventID"] = &types.AttributeValueMemberS{Value: uuid_new}
	//-----------------------------------------GET QUERY LOCATION FIELD-----------------------------------------
	if val, ok := search["location"].(map[string]interface{}); ok {
		long, ok2 := val["BuildingLong"].(float64)
		lat, ok3 := val["BuildingLat"].(float64)

		if !ok2 || !ok3 {
			return Helpers.ResponseGeneration("location schema missing BuildingLong and/or BuildingLat", http.StatusOK)
		}

		slong := strconv.FormatFloat(long, 'f', -1, 64)
		slat := strconv.FormatFloat(lat, 'f', -1, 64)
		item["locationQueryID"] = &types.AttributeValueMemberS{Value: (slong + " " + slat)}
	} else {
		return Helpers.ResponseGeneration("missing location", http.StatusOK)
	}

	//-----------------------------------------MAKE TTL VALUE-----------------------------------------
	expire := -2
	if val, ok := search["expires"].(float64); ok {
		expire = int(val)
	}
	makeTTL(item, search, expire)
	//-----------------------------------------GET KEYS TO FILTER-----------------------------------------
	keys := make(map[string]types.AttributeValue)
	keys[":EventsID"] = &types.AttributeValueMemberS{Value: uuid_new}

	if errs != nil {
		return Helpers.ResponseGeneration(errs.Error(), http.StatusBadRequest)
	}

	//-----------------------------------------PUT INTO DATABASE-----------------------------------------
	_, err = client.PutItem(context.Background(), &dynamodb.PutItemInput{
		ExpressionAttributeValues: keys,
		TableName:                 aws.String(table),
		Item:                      item,
		ConditionExpression:       aws.String("EventsID <> :EventsID"),
	})

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
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
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
