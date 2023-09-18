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
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type LocationQuery struct {
	Radius float64 `json:"radius"`
	Long   float64 `json:"long"`
	Lat    float64 `json:"lat"`
}

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
	search := LocationQuery{}

	err = json.Unmarshal([]byte(request.Body), &search)

	if err != nil {
		return Helpers.ResponseGeneration("missing field", http.StatusOK)
	}

	//-----------------------------------------GET CALCULATIONS-----------------------------------------
	calculations := make(map[string]interface{})
	calculations[":MinLat"] = search.Lat - search.Radius
	calculations[":MaxLat"] = search.Lat + search.Radius

	calculations[":MinLong"] = search.Long - search.Radius
	calculations[":MaxLong"] = search.Long + search.Radius

	calc_attr, err := attributevalue.MarshalMap(calculations)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}
	//-----------------------------------------BUILD QUERY-----------------------------------------
	query := "BuildingLong BETWEEN :MinLong AND :MaxLong AND BuildingLat BETWEEN :MinLat AND :MaxLat"

	//-----------------------------------------PUT INTO DATABASE-----------------------------------------
	scanInput := &dynamodb.ScanInput{
		// table name is a global variable
		TableName:                 &table,
		ExpressionAttributeValues: calc_attr,
		FilterExpression:          &query,
	}

	paginator := dynamodb.NewScanPaginator(client, scanInput)
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

	square_results, err := produceQueryResult(paginator)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------FILTER BASED ON CIRCULAR RANGE-----------------------------------------
	ret["results"] = filterByRadius(square_results, search.Radius, search.Lat, search.Long)

	js, err := json.Marshal(ret)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
