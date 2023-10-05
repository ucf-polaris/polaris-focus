package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"polaris-api/pkg/Helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
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

func pointInRadius(radius float64, lat float64, long float64, myLat float64, myLong float64) bool {
	return math.Sqrt(math.Pow(myLat-lat, 2)+math.Pow(myLong-long, 2)) <= radius
}

func filterByRadius(M []map[string]interface{}, radius float64, lat float64, long float64) []map[string]interface{} {
	ret := []map[string]interface{}{}
	for _, e := range M {
		myLat, _ := e["BuildingLat"].(float64)
		myLong, _ := e["BuildingLong"].(float64)

		if pointInRadius(radius, lat, long, myLat, myLong) {
			ret = append(ret, e)
		}
	}

	return ret
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

	js, err := json.Marshal(ret)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
