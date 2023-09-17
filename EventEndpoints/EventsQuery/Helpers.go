package main

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

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
