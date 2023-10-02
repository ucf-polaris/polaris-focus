package main

import (
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	lambdaCall "github.com/aws/aws-sdk-go/service/lambda"
)

var lambdaClient *lambdaCall.Lambda
var table string
var client *dynamodb.Client

func init() {
	//dynamo db
	client, table = Helpers.ConstructDynamoHost()
}

func main() {
	lambda.Start(handler)
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	res, err := Helpers.QueryKey(
		client,
		map[string]interface{}{
			"email": "kaedenle@gmail.com",
			"name":  "kaeden",
		},
		table,
		[]string{"UserID", "OtherID"},
	)
	if err != nil {
		panic(err)
	}
	log.Println(res)
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: " ", Headers: map[string]string{"content-type": "application/json"}}, nil
}
