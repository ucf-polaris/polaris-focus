package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"

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

func extractFields(fields map[string]string, mapping map[string]interface{}, addColon bool) (string, map[string]types.AttributeValue, error) {
	flag := true
	failRet := ""
	item := make(map[string]types.AttributeValue)

	for k, v := range fields {
		if !containsKey(mapping, k) {
			flag = false
			if failRet != "" {
				failRet += " "
			}
			failRet += k

		} else {
			var key string
			if addColon {
				key = ":" + k
			} else {
				key = k
			}
			switch val := mapping[k].(type) {
			case float64:
				if v != "N" {
					return "", nil, errors.New(k + " is not supposed to be float")
				}
				item[key] = &types.AttributeValueMemberN{Value: strconv.FormatFloat(val, 'f', -1, 64)}
			case int:
				if v != "N" {
					return "", nil, errors.New(k + " is not supposed to be number")
				}
				item[key] = &types.AttributeValueMemberN{Value: strconv.Itoa(val)}
			case bool:
				if v != "BOOL" {
					return "", nil, errors.New(k + " is not supposed to be bool")
				}
				item[key] = &types.AttributeValueMemberBOOL{Value: val}
			case string:
				if v != "S" {
					return "", nil, errors.New(k + " is not supposed to be string")
				}
				item[key] = &types.AttributeValueMemberS{Value: val}
			case []interface{}:
				if v != "L" {
					return "", nil, errors.New(k + " is not supposed to be list")
				}
				av, err := attributevalue.MarshalList(val)
				if err != nil {
					panic(err)
				}
				item[key] = &types.AttributeValueMemberL{Value: av}
			default:
				typing := reflect.TypeOf(val).Elem().String()
				str := fmt.Sprintf("%v", val)
				return "", nil, errors.New("type not recognized " + typing + ". Value is " + str + ". On field " + k)
			}
		}
	}
	if !flag {
		return failRet, item, nil
	} else {
		return "", item, nil
	}

}

func responseGeneration(msg string, status int) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{StatusCode: status, Body: "Error: " + msg}, nil
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

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------EXTRACT TOKEN FIELDS-----------------------------------------
	var token string
	var rfsTkn string

	if request.RequestContext.Authorizer != nil {
		contextString := request.RequestContext.Authorizer["stringKey"].(string)

		ctxt := map[string]any{}
		err := json.Unmarshal([]byte(contextString), &ctxt)
		if err != nil {
			return responseGeneration(err.Error(), http.StatusBadRequest)
		}

		if val, ok := ctxt["token"].(string); ok {
			token = val
		}

		if val, ok := ctxt["refreshToken"].(string); ok {
			rfsTkn = val
		}
	}
	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	search := unpackRequest(request.Body)

	errOutput, item, errs := extractFields(
		map[string]string{"BuildingLong": "N", "BuildingLat": "N", "BuildingDesc": "S", "BuildingEvents": "L", "BuildingName": "S"},
		search,
		false)

	if errs != nil {
		return responseGeneration(errs.Error(), http.StatusBadRequest)
	}

	if errOutput != "" {
		return responseGeneration("field not set ("+errOutput+")", http.StatusBadRequest)
	}
	//-----------------------------------------GET KEYS TO FILTER-----------------------------------------
	_, keys, errs := extractFields(
		map[string]string{"BuildingLong": "N", "BuildingLat": "N"},
		search,
		true)

	if errs != nil {
		return responseGeneration(errs.Error(), http.StatusBadRequest)
	}
	//-----------------------------------------PUT INTO DATABASE-----------------------------------------

	_, err := client.PutItem(context.Background(), &dynamodb.PutItemInput{
		ExpressionAttributeValues: keys,
		TableName:                 aws.String(table),
		Item:                      item,
		ConditionExpression:       aws.String("BuildingLong <> :BuildingLong AND BuildingLat <>  :BuildingLat"),
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
