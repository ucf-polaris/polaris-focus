package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
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

func containsKey[M ~map[K]V, K comparable, V any](m M, k K) bool {
	_, ok := m[k]
	return ok
}

func produceErrorMsg(fields []string, mapping map[string]interface{}) string {
	var failRet string
	for _, element := range fields {
		if !containsKey(mapping, element) {
			//if allOptional is true, ignore below and don't throw error for missing a field (all of them are optional)
			if failRet != "" {
				failRet += " "
			}
			failRet += element
		}
	}
	return failRet
}

func addColonToField(mapping map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for k, v := range mapping {
		ret[":"+k] = v
	}

	return ret
}

func createMap(fields []string, mapping map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for _, element := range fields {
		if containsKey(mapping, element) {
			ret[element] = mapping[element]
		}
	}
	return ret
}

func buildQuery(fields []string, mapping map[string]interface{}) (string, map[string]string) {
	ret := ""
	ret_map := make(map[string]string)

	for _, element := range fields {
		if containsKey(mapping, element) {
			if ret != "" {
				ret += ", "
			}
			ret += ("#" + element + " = " + ":" + element)
			ret_map["#"+element] = element
		}
	}

	ret = "set " + ret

	return ret, ret_map
}

func extractFields(fields []string, mapping map[string]interface{}, addColon bool, allOptional bool) (map[string]types.AttributeValue, string, map[string]string, error) {
	failRet := ""
	new_map := createMap(fields, mapping)

	//check if missing fields
	if !allOptional {
		failRet = produceErrorMsg(fields, new_map)
	}

	using_map := new_map
	if addColon {
		using_map = addColonToField(new_map)
	}

	item, err := attributevalue.MarshalMap(using_map)
	if err != nil {
		return nil, "", nil, err
	}

	query, mapQuery := buildQuery(fields, new_map)

	//return situations
	//1. allOptional is false and there's a missing field
	//2. the return is empty
	//3. successful
	if failRet != "" {
		return nil, "", nil, errors.New("items not in request: " + failRet)
	} else if len(item) == 0 {
		return nil, "", nil, errors.New("no items found")
	} else {
		return item, query, mapQuery, nil
	}

}

func responseGeneration(errMsg string, status int) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{StatusCode: status, Body: "ERROR " + errMsg}, nil
}

func unpackRequest(body string) map[string]interface{} {
	if body == "" {
		return nil
	}

	log.Println("body: ", body)

	search := map[string]any{}
	err := json.Unmarshal([]byte(body), &search)

	if err != nil {
		panic(err)
	}

	return search
}

func getTokens(request events.APIGatewayProxyRequest) (string, string, error) {
	var token string
	var rfsTkn string

	if request.RequestContext.Authorizer != nil {
		contextString := request.RequestContext.Authorizer["stringKey"].(string)

		ctxt := map[string]any{}
		err := json.Unmarshal([]byte(contextString), &ctxt)
		if err != nil {
			return "", "", nil
		}

		if val, ok := ctxt["token"].(string); ok {
			token = val
		}

		if val, ok := ctxt["refreshToken"].(string); ok {
			rfsTkn = val
		}
	}

	return token, rfsTkn, nil
}

func makeTTL(item map[string]types.AttributeValue, search map[string]interface{}, expire int) error {
	date, _ := search["dateTime"].(string)
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
