package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

type LocationQuery struct {
	Long float64 `json:"long"`
	Lat  float64 `json:"lat"`
}

var table string
var client *dynamodb.Client

func init() {
	client, table = Helpers.ConstructDynamoHost()

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}
}

func main() {
	lambda.Start(handler)
}

func produceQueryResult(page *dynamodb.QueryPaginator) ([]map[string]interface{}, error) {
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

	item := make(map[string]types.AttributeValue)
	item[":locationQueryID"] = &types.AttributeValueMemberS{Value: (strconv.FormatFloat(search.Long, 'f', -1, 64) + " " + strconv.FormatFloat(search.Lat, 'f', -1, 64))}
	//-----------------------------------------PUT INTO DATABASE-----------------------------------------
	queryInput := &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		IndexName:                 aws.String("locationQueryID-index"),
		KeyConditionExpression:    aws.String("locationQueryID = :locationQueryID"),
		ExpressionAttributeValues: item,
	}

	paginator := dynamodb.NewQueryPaginator(client, queryInput)
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

	ret["results"], err = produceQueryResult(paginator)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	ret["tokens"] = tokens

	js, err := json.Marshal(ret)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
