package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"polaris-api/pkg/Helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

var table string
var counter string
var client *dynamodb.Client

type Counter struct {
	Name string `json:"ID"`
}

type CounterAttribute struct {
	Name string `json:":ID"`
	Inc  int    `json:":inc"`
}

func init() {
	//dynamo db
	client, table = Helpers.ConstructDynamoHost()

	counter = os.Getenv("COUNTER_NAME")
}

func main() {
	lambda.Start(handler)
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	c := Counter{
		Name: counter,
	}

	ca := CounterAttribute{
		Name: counter,
		Inc:  1,
	}

	key, _ := attributevalue.MarshalMap(c)
	items, _ := attributevalue.MarshalMap(ca)

	updateInput := &dynamodb.UpdateItemInput{
		// table name is a global variable
		TableName: &table,
		// Partitiion key for user table is EventID
		Key: key,
		// "SET" update expression to update the item in the table.
		UpdateExpression:          aws.String("ADD Counter :inc"),
		ExpressionAttributeValues: items,
		ReturnValues:              types.ReturnValueUpdatedNew,
		//don't make new record if key doesn't exist
		ConditionExpression: aws.String("EventID = :ID"),
	}

	retValues, err := client.UpdateItem(context.Background(), updateInput)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	js, err := json.Marshal(retValues.Attributes)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
