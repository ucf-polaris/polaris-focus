package Helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	lambdaCall "github.com/aws/aws-sdk-go/service/lambda"
)

var (
	ErrRecordNotFound = fmt.Errorf("record not found")
	ErrKeyNotFound    = fmt.Errorf("key not found")
)

type Counter struct {
	Name string `dynamodbav:"ID"`
}

type CounterAttribute struct {
	Name string `dynamodbav:":ID"`
	Inc  int    `dynamodbav:":inc"`
}

type CounterRet struct {
	Counter int `json:"Counter"`
}

// checks if key is in map[string]interface{}
func containsKey(m map[string]interface{}, k string) bool {
	_, ok := m[k]
	return ok
}

// produces string error message of fields not in request. Links to ExtractFields
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

// produces an interface with colons at the front of the keys to prepare for attributevalue mapping. Links back to ExtractFields
func addColonToField(mapping map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for k, v := range mapping {
		ret[":"+k] = v
	}

	return ret
}

// culls down mapping based on whats within fields. Links back to ExtractFields
func createMap(fields []string, mapping map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for _, element := range fields {
		if containsKey(mapping, element) {
			ret[element] = mapping[element]
		}
	}
	return ret
}

// builds equality query for DynamoDB. Links back to ExtractFields.
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

// Gets needed elements for a query, get, add, or update.
func ExtractFields(fields []string, mapping map[string]interface{}, addColon bool, allOptional bool) (map[string]types.AttributeValue, string, map[string]string, error) {
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
	} else if len(item) == 0 && !allOptional {
		return nil, "", nil, errors.New("no items found")
	} else {
		return item, query, mapQuery, nil
	}

}

// shortcut to produce a quick response
func ResponseGeneration(errMsg string, status int) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{StatusCode: status, Body: "ERROR " + errMsg}, nil
}

// unpacks request body from string into an interface (handles errors)
func UnpackRequest(body string) map[string]interface{} {
	if body == "" {
		return nil
	}

	//log.Println("body: ", body)

	search := map[string]any{}
	err := json.Unmarshal([]byte(body), &search)

	if err != nil {
		panic(err)
	}

	return search
}

// gets tokens from authorizer (if it exists)
func GetTokens(request events.APIGatewayProxyRequest) (string, string, error) {
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

// constructs Dynamo host, local or in lambda
func ConstructDynamoHost() (*dynamodb.Client, string) {
	var err error
	var cfg aws.Config
	var table_func string

	//if local
	if IsLambdaLocal() {
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion("localhost"),
			config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: "http://localhost:8000"}, nil
				})),
			config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
				Value: aws.Credentials{
					AccessKeyID: "abcd", SecretAccessKey: "a1b2c3", SessionToken: "",
					Source: "Mock credentials used above for local instance",
				},
			}),
		)
		if err != nil {
			panic(err)
		}
		table_func = "THENEWTABLE"
		//if not local
	} else {
		cfg, err = config.LoadDefaultConfig(context.Background())
		if err != nil {
			panic(err)
		}
		table_func = os.Getenv("TABLE_NAME")
	}

	return dynamodb.NewFromConfig(cfg), table_func
}

// override to create an actual host
func ConstructRealDynamoHost() (*dynamodb.Client, string) {
	var err error
	var cfg aws.Config
	var table_func string

	cfg, err = config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}
	table_func = os.Getenv("TABLE_NAME")

	return dynamodb.NewFromConfig(cfg), table_func
}

// determines if in local environment based on existance of named testing file
func IsLambdaLocal() bool {
	test := os.Getenv("LAMBDA_TASK_ROOT")
	return test == ""
}

// transforms all fields provided into string set from lists
func ListToStringSet(fields []string, M map[string]types.AttributeValue, returnError bool) error {
	//go through fields
	for _, element := range fields {
		//if of type AV list
		if val, ok := M[element].(*types.AttributeValueMemberL); ok {

			temp := []string{}
			err := attributevalue.Unmarshal(val, &temp)

			if err != nil {
				panic(err)
			}

			//delete key if empty (ADD will reappend)
			if len(temp) != 0 {
				M[element] = &types.AttributeValueMemberSS{Value: temp}
			} else {
				delete(M, element)
				if returnError {
					return errors.New("empty list passed in, set cannot be empty")
				}
			}
		}
	}
	return nil
}

func CreateToken(lambdaClient *lambdaCall.Lambda, timeTil int, userID string, mode float64) (string, error) {
	//-----------------------------------------GET VARIABLES-----------------------------------------
	JWTFields := make(map[string]interface{})

	JWTFields["timeTil"] = timeTil
	JWTFields["mode"] = mode

	if userID != "" {
		JWTFields["UserID"] = userID
	}
	//-----------------------------------------PACKAGE RESPONSE-----------------------------------------
	payload, err := json.Marshal(JWTFields)
	if err != nil {
		return "", err
	}

	resultToken, err := lambdaClient.Invoke(&lambdaCall.InvokeInput{FunctionName: aws.String("token_create"), Payload: payload})
	if err != nil {
		return "", err
	}

	result_json := UnpackRequest(string(resultToken.Payload))

	token := result_json["token"].(string)

	return token, nil
}

func MergeMaps(M1 map[string]types.AttributeValue, M2 map[string]types.AttributeValue) map[string]types.AttributeValue {
	for k, v := range M2 {
		M1[k] = v
	}
	return M1
}

func QueryKey(client *dynamodb.Client, index_map map[string]interface{}, table string, keys []string) ([]map[string]interface{}, error) {
	query := ""
	index_name := ""
	name_express := map[string]string{}
	projection := ""

	//construct query and index name
	for k, _ := range index_map {
		if query != "" {
			query += " AND "
		}

		query += ("#" + k + " = :" + k)

		index_name += (k + "-")

		name_express["#"+k] = k
	}

	for _, val := range keys {
		if projection != "" {
			projection += ", "
		}
		projection += "#" + val
		name_express["#"+val] = val
	}

	index_name += "index"

	index_map = addColonToField(index_map)
	using_map, _ := attributevalue.MarshalMap(index_map)

	QueryResults, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		IndexName:                 aws.String(index_name),
		KeyConditionExpression:    aws.String(query),
		ExpressionAttributeValues: using_map,
		ExpressionAttributeNames:  name_express,
		ProjectionExpression:      &projection,
	})
	//-----------------------------------------ERROR CHECKING-----------------------------------------
	//General error occured
	if err != nil {
		return nil, err
	}

	//No username found
	if QueryResults.Count == 0 {
		return nil, errors.New("no keys found")
	}

	output := []map[string]interface{}{}
	attributevalue.UnmarshalListOfMaps(QueryResults.Items, &output)

	return output, nil
}

func IncrementCounterTable(client *dynamodb.Client, counter, table string) (int, error) {
	c := Counter{
		Name: counter,
	}

	ca := CounterAttribute{
		Name: counter,
		Inc:  1,
	}

	name := map[string]string{
		"#Counter": "Counter",
	}

	key, _ := attributevalue.MarshalMap(c)
	items, _ := attributevalue.MarshalMap(ca)
	log.Println(key)

	updateInput := &dynamodb.UpdateItemInput{
		// table name is a global variable
		TableName: &table,
		// Partitiion key for user table is EventID
		Key: key,
		// "SET" update expression to update the item in the table.
		UpdateExpression:          aws.String("ADD #Counter :inc"),
		ExpressionAttributeValues: items,
		ExpressionAttributeNames:  name,
		ReturnValues:              types.ReturnValueUpdatedNew,
		//don't make new record if key doesn't exist
		ConditionExpression: aws.String("ID = :ID"),
	}

	retValues, err := client.UpdateItem(context.Background(), updateInput)
	if err != nil {
		return 0, err
	}

	ret := CounterRet{}
	err = attributevalue.UnmarshalMap(retValues.Attributes, &ret)
	if err != nil {
		return 0, err
	}

	return ret.Counter, nil
}

func GetCounterTable(client *dynamodb.Client, counter, table string) (int, error) {
	c := Counter{
		Name: counter,
	}

	key, _ := attributevalue.MarshalMap(c)

	getInput := &dynamodb.GetItemInput{
		// table name is a global variable
		TableName: &table,
		// Partitiion key for user table is EventID
		Key: key,
	}

	retValues, err := client.GetItem(context.Background(), getInput)
	if err != nil {
		return 0, err
	}

	ret := CounterRet{}
	err = attributevalue.UnmarshalMap(retValues.Item, &ret)
	if err != nil {
		return 0, err
	}

	return ret.Counter, nil
}
