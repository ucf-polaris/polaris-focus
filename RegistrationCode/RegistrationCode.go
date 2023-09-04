package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
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
	cfg, _ := config.LoadDefaultConfig(context.Background())
	client = dynamodb.NewFromConfig(cfg)
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

func responseGeneration(msg string, status int) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{StatusCode: status, Body: "Error: " + msg}, nil
}

func main() {
	lambda.Start(handler)
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	body := unpackRequest(request.Body)
	log.Println(request.Body)
	var code float64
	var ok bool
	var contextString string
	var userID string

	if request.RequestContext.Authorizer != nil {
		contextString = request.RequestContext.Authorizer["stringKey"].(string)
		log.Println(contextString)
	}

	ctxt := map[string]any{}
	json.Unmarshal([]byte(contextString), &ctxt)

	//look for email from JWT first, if not there look in passed in body
	if userID, ok = ctxt["UserID"].(string); !ok {
		if userID, ok = body["UserID"].(string); !ok {
			return responseGeneration("UserID field not set", http.StatusBadRequest)
		}
	}

	if code, ok = body["code"].(float64); !ok {
		return responseGeneration("code field not set", http.StatusBadRequest)
	}

	codeStr := strconv.Itoa(int(code))

	//-----------------------------------------THE UPDATE CALL-----------------------------------------
	//pass changes into update
	item := make(map[string]types.AttributeValue)
	item[":code"] = &types.AttributeValueMemberN{Value: codeStr}

	//who we're trying to find
	key := make(map[string]types.AttributeValue)
	key["UserID"] = &types.AttributeValueMemberS{Value: userID}

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeValues: item,
		TableName:                 aws.String(table),
		Key:                       key,
		ReturnValues:              types.ReturnValueAllNew,
		UpdateExpression:          aws.String("remove timeTilExpire, verificationCode"),
		ConditionExpression:       aws.String("verificationCode = :code"),
	}

	output, err := client.UpdateItem(context.Background(), input)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	map_output := map[string]any{}
	attributevalue.UnmarshalMap(output.Attributes, &map_output)
	delete(map_output, "password")
	log.Println(map_output)

	if len(map_output) == 0 {
		return responseGeneration("code incorrect", http.StatusBadRequest)
	}

	js, err := json.Marshal(map_output)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
