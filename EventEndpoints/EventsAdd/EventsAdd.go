package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"
	"strconv"
	"time"

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

// make a TTL
func makeTTL(item map[string]types.AttributeValue, search map[string]interface{}, expire int) error {
	date, _ := search["endsOn"].(string)
	thetime, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return err
	}

	var timeVal string
	if expire == -2 {
		timeVal = strconv.FormatInt(thetime.UTC().Add(time.Hour*24).Unix(), 10)
	} else if expire <= 0 {
		timeVal = "0"
	} else {
		timeVal = strconv.FormatInt(thetime.UTC().Add(time.Hour*time.Duration(expire)).Unix(), 10)
	}

	//make sure dates aren't older than the current day (or by 5 years)
	item["timeTilExpire"] = &types.AttributeValueMemberN{Value: timeVal}

	return nil
}

func produceUUID() string {
	//allows unit testing to be consistent
	if Helpers.IsLambdaLocal() {
		return "0"
	}
	return uuid.Must(uuid.NewRandom()).String()
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------EXTRACT TOKEN FIELDS-----------------------------------------
	token, rfsTkn, err := Helpers.GetTokens(request)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}
	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	search := Helpers.UnpackRequest(request.Body)

	log.Println(search)

	item, _, _, errs := Helpers.ExtractFields(
		[]string{"name", "host", "description", "dateTime", "location", "endsOn", "image", "listedLocation"},
		search,
		false,
		false)

	if errs != nil {
		return Helpers.ResponseGeneration(errs.Error(), http.StatusOK)
	}

	var uuid_new string
	if val, ok := search["EventID"].(string); ok {
		uuid_new = val
	} else {
		uuid_new = produceUUID()
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
		return Helpers.ResponseGeneration("location schema missing BuildingLong and/or BuildingLat", http.StatusOK)
	}

	//-----------------------------------------MAKE TTL VALUE-----------------------------------------
	expire := 1440
	if val, ok := search["expires"].(float64); ok {
		expire = int(val)
	}
	makeTTL(item, search, expire)
	//-----------------------------------------GET KEYS TO FILTER-----------------------------------------
	keys := make(map[string]types.AttributeValue)
	keys[":EventID"] = &types.AttributeValueMemberS{Value: uuid_new}

	if errs != nil {
		return Helpers.ResponseGeneration(errs.Error(), http.StatusOK)
	}
	//-----------------------------------------PUT INTO DATABASE-----------------------------------------

	_, err = client.PutItem(context.Background(), &dynamodb.PutItemInput{
		ExpressionAttributeValues: keys,
		TableName:                 aws.String(table),
		Item:                      item,
		ConditionExpression:       aws.String("EventID <> :EventID"),
	})

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
	ret["EventID"] = uuid_new

	js, err := json.Marshal(ret)

	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
