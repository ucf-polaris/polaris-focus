package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var table string
var client *dynamodb.Client

func init() {
	client, table = Helpers.ConstructDynamoHost()

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}
}

func produceQueryResult(page *dynamodb.ScanPaginator) ([]map[string]interface{}, error) {
	p := []map[string]interface{}{}

	for page.HasMorePages() {
		out, err := page.NextPage(context.TODO())
		if err != nil {
			return nil, err
		}

		temp := []map[string]interface{}{}
		err = attributevalue.UnmarshalListOfMaps(out.Items, &temp)
		if err != nil {
			return nil, err
		}

		p = append(p, temp...)
	}

	return p, nil
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

	//-----------------------------------------PUT INTO DATABASE-----------------------------------------
	scanInput := &dynamodb.ScanInput{
		// table name is a global variable
		TableName: &table,
	}

	paginator := dynamodb.NewScanPaginator(client, scanInput)
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

	res, err := produceQueryResult(paginator)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------FILTER BASED ON CIRCULAR RANGE-----------------------------------------
	ret["locations"] = res

	if table == "Events" {
		counter, err := Helpers.GetCounterTable(client, "EventParseAmount", "Counters")
		if err != nil {
			return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
		}
		ret["counter"] = counter
	}

	js, err := json.Marshal(ret)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
