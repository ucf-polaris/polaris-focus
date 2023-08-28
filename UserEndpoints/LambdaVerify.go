// Early Proof of Concept for verification of users
// Implement:
// * sending back the JWT token and Refresh Token
package main

import (
	"context"
	"encoding/json"
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

type User struct {
	UserID   string   `json:"UserID"`
	Email    string   `json:"email"`
	Password string   `json:"password,omitempty"`
	Schedule []string `json:"schedule"`
	Username string   `json:"username"`
	Name     string   `json:"name"`
}

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

func main() {
	lambda.Start(handler)
}

func unpackRequest(body string) User {
	if body == "" {
		return User{}
	}

	log.Println("body: ", body)

	search := User{}
	err := json.Unmarshal([]byte(body), &search)

	if err != nil {
		panic(err)
	}

	return search
}

func responseGeneration(msg string, status int) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{StatusCode: status, Body: "Error: " + msg}
}

// TO-DO: create a function that handles response returns (more clean and more info/debug info)
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	search := unpackRequest(request.Body)

	if search.Username == "" || search.Password == "" {
		return responseGeneration("field not set", http.StatusBadRequest), nil
	}
	item_username := make(map[string]types.AttributeValue)
	item_username[":username"] = &types.AttributeValueMemberS{Value: search.Username}

	TheInput, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		IndexName:                 aws.String("username-index"),
		KeyConditionExpression:    aws.String("username = :username"),
		ExpressionAttributeValues: item_username,
	})

	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest), nil
	}

	if TheInput.Count == 0 {
		return responseGeneration("invalid username/password", http.StatusBadRequest), nil
	}

	if TheInput.Count > 1 {
		return responseGeneration("more than one username found", http.StatusBadRequest), nil
	}

	newUser := User{}

	attributevalue.UnmarshalMap(TheInput.Items[0], &newUser)

	check_pass := newUser.Password
	newUser.Password = ""

	js, err := json.Marshal(newUser)

	if err != nil {
		return responseGeneration(err.Error(), http.StatusBadRequest), nil
	}

	//3. Return whole body if it is correct, if not return nothing!
	if check_pass != search.Password {
		return responseGeneration("invalid username/password", http.StatusBadRequest), nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
