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
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaCall "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	UserID string `json:"userID,omitempty"`
	jwt.RegisteredClaims
}

var table string
var client *dynamodb.Client
var lambdaClient *lambdaCall.Lambda

func init() {
	//dynamo stuff
	table = os.Getenv("TABLE_NAME")

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}

	cfg, _ := config.LoadDefaultConfig(context.Background())
	client = dynamodb.NewFromConfig(cfg)

	//set up lambda client
	//create session for lambda
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	lambdaClient = lambdaCall.New(sess, &aws.Config{Region: aws.String("us-east-2")})

}

func createToken(timeTil int, userID string, mode float64) (string, error) {
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

	result_json := unpackRequest(string(resultToken.Payload))

	token := result_json["token"].(string)

	return token, nil
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

	//-----------------------------------------RESULTS PROCESSING-----------------------------------------
	map_output := map[string]any{}
	attributevalue.UnmarshalMap(output.Attributes, &map_output)
	delete(map_output, "password")

	log.Println(map_output)

	if len(map_output) == 0 {
		return responseGeneration("code incorrect", http.StatusBadRequest)
	}

	//-----------------------------------------CREATE TOKENS-----------------------------------------
	map_output["token"], err = createToken(15, "", 0)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	map_output["refreshToken"], err = createToken(1440, "", 1)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	js, err := json.Marshal(map_output)
	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
