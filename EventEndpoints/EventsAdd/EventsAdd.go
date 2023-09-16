package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/uuid"
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
	token, rfsTkn, err := getTokens(request)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}
	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	search := unpackRequest(request.Body)

	item, _, _, errs := extractFields(
		[]string{"name", "host", "description", "dateTime", "location"},
		search,
		false,
		false)

	if errs != nil {
		return responseGeneration(errs.Error(), http.StatusBadRequest)
	}

	uuid_new := uuid.Must(uuid.NewRandom()).String()
	item["EventID"] = &types.AttributeValueMemberS{Value: uuid_new}

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
		return responseGeneration(errs.Error(), http.StatusBadRequest)
	}
	//-----------------------------------------PUT INTO DATABASE-----------------------------------------

	_, err = client.PutItem(context.Background(), &dynamodb.PutItemInput{
		ExpressionAttributeValues: keys,
		TableName:                 aws.String(table),
		Item:                      item,
		ConditionExpression:       aws.String("EventsID <> :EventsID"),
	})

	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
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
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
