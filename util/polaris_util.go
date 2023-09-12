package polaris_util

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"
	"strconv"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-lambda-go/events"
)

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
			case interface{}:
				if v == "M" {
					av, err := attributevalue.MarshalMap(val)
					if err != nil {
						panic(err)
					}
					item[key] = &types.AttributeValueMemberM{Value: av}
				} else {
					typing := reflect.TypeOf(val).Elem().String()
					str := fmt.Sprintf("%v", val)
					log.Println("type not recognized " + typing + ". Value is " + str + ". On field " + k)
				}

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

func inTimeSpan(start, end, check time.Time) bool {
	return check.After(start) && check.Before(end)
}

func mapEnforceSchema(m map[string]interface{}, schema []string) error {

	//match up the schema
	for _, element := range schema {
		_, ok := m[element]
		if !ok {
			return errors.New(element + " not found")
		}
	}

	//if get past schema match, test for any foreign keys (look at length)
	if len(schema) != len(m) {
		return errors.New("foreign keys detected in map")
	}

	return nil
}
