package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
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
	UserID       string   `json:"UserID"`
	Email        string   `json:"email"`
	Password     string   `json:"password"`
	Schedule     []string `json:"schedule"`
	Username     string   `json:"username"`
	Token        string   `json:"token"`
	RefreshToken string   `json:"refreshToken,omitempty"`
}

type UserSearch struct {
	UserID   string `json:"UserID"`
	UseUser  bool   `json:"useUser"`
	Username string `json:"username"`
}

type JWTPackage struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken,omitempty"`
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

/*func handler(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {

	payload := event.Body
	log.Println("payloads", payload)

	return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusOK}, nil
}*/

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	search := UserSearch{}
	log.Println(request.Body)

	var s string
	if request.RequestContext.Authorizer != nil {
		s = request.RequestContext.Authorizer["stringKey"].(string)
		log.Println(s)
	}

	pack := JWTPackage{}
	json.Unmarshal([]byte(s), &pack)

	if request.Body == "" {
		return events.APIGatewayProxyResponse{Body: request.Body}, errors.New("no body found")
	}

	err := json.Unmarshal([]byte(request.Body), &search)

	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	log.Println("payloads: ", search)
	item := make(map[string]types.AttributeValue)

	item["UserID"] = &types.AttributeValueMemberS{Value: search.UserID}

	TheInput, err := client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key:       item,
	})

	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	newUser := User{}

	attributevalue.UnmarshalMap(TheInput.Item, &newUser)

	newUser.Token = pack.Token
	if pack.RefreshToken != "" {
		newUser.RefreshToken = pack.RefreshToken
	}

	js, err := json.Marshal(newUser)

	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	log.Println(newUser)
	response := events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(js),
	}

	return response, nil
}
