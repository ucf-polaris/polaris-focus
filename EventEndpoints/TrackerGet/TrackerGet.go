package main

import (
	"encoding/json"
	"net/http"
	"os"
	"polaris-api/pkg/Helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var table string
var counter string
var client *dynamodb.Client

func init() {
	//dynamo db
	client, table = Helpers.ConstructDynamoHost()
	counter = os.Getenv("COUNTER_NAME")
}

func main() {
	lambda.Start(handler)
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	counter, err := Helpers.GetCounterTable(client, counter, table)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	ret := map[string]interface{}{
		"counter": counter,
	}

	js, err := json.Marshal(ret)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
