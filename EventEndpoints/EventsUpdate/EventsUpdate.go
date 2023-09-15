package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

var table string
var client *dynamodb.Client

func init() {
	table = os.Getenv("TABLE_NAME")

	/*if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}*/

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

func responseGeneration(err error, status int) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{StatusCode: status, Body: ""}, err
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

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------EXTRACT TOKEN FIELDS-----------------------------------------
	token, rfsTkn, err := getTokens(request)
	if err != nil {
		return responseGeneration(err, http.StatusOK)
	}

	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	search := unpackRequest(request.Body)

	items, queryString, mapQuery, err := extractFields(
		[]string{"name", "host", "description", "dateTime", "location"},
		search,
		true,
		true)

	if err != nil {
		return responseGeneration(err, http.StatusOK)
	}
	//-----------------------------------------GET KEYS TO FILTER-----------------------------------------
	key, _, _, err := extractFields(
		[]string{"EventID"},
		search,
		false,
		false)
	if err != nil {
		return responseGeneration(err, http.StatusOK)
	}

	//put key in ExpressionAttributeValues for ConditionExpression
	items[":EventID"] = key["EventID"]
	//-----------------------------------------PUT INTO DATABASE-----------------------------------------
	updateInput := &dynamodb.UpdateItemInput{
		// table name is a global variable
		TableName: &table,
		// Partitiion key for user table is EventID
		Key: key,
		// "SET" update expression to update the item in the table.
		UpdateExpression:          aws.String(queryString),
		ExpressionAttributeNames:  mapQuery,
		ExpressionAttributeValues: items,
		ReturnValues:              types.ReturnValueUpdatedNew,
		//don't make new record if key doesn't exist (could take this out and make a new add?)
		ConditionExpression: aws.String("EventID = :EventID"),
	}

	retValues, err := client.UpdateItem(context.Background(), updateInput)
	if err != nil {
		return responseGeneration(err, http.StatusOK)
	}

	//-----------------------------------------PACK RETURN VALUES-----------------------------------------
	ret := make(map[string]interface{})
	attributevalue.UnmarshalMap(retValues.Attributes, &ret)
	if token != "" {
		ret["token"] = token
	}

	if rfsTkn != "" {
		ret["refreshToken"] = rfsTkn
	}

	js, err := json.Marshal(ret)
	if err != nil {
		return responseGeneration(err, http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
