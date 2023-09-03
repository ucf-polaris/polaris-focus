package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
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

type User struct {
	UserID           string   `json:"UserID"`
	Email            string   `json:"email"`
	Password         string   `json:"password"`
	Schedule         []string `json:"schedule"`
	Username         string   `json:"username"`
	Name             string   `json:"name"`
	TimeTilExpire    int      `json:"timeTilExpire"`
	VerificationCode int      `json:"VerificationCode"`
}

type UserSearch struct {
	UserID   string `json:"UserID"`
	UseUser  bool   `json:"useUser"`
	Username string `json:"username"`
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
	/*myuuid := uuid.Must(uuid.NewRandom()).String()

	fmt.Println(myuuid)*/
	lambda.Start(handler)
	//fmt.Println(produceRandomNDigits(5))
}

func produceRandomNDigits(N int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var number string

	for i := 0; i < N; i++ {
		digit := r.Intn(10)
		number += strconv.Itoa(digit)
	}

	return number
}

func handler(event User) (events.APIGatewayV2HTTPResponse, error) {

	if event.Email == "" || event.Password == "" || event.Username == "" {
		return events.APIGatewayV2HTTPResponse{}, errors.New("field not set")
	}

	item_email := make(map[string]types.AttributeValue)
	item_email[":email"] = &types.AttributeValueMemberS{Value: event.Email}

	TheInput, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		IndexName:                 aws.String("email-index"),
		KeyConditionExpression:    aws.String("email = :email"),
		ExpressionAttributeValues: item_email,
	})

	if err != nil {
		panic(err)
	}

	if TheInput.Count != 0 {
		return events.APIGatewayV2HTTPResponse{}, errors.New("email already in use")
	}

	item := make(map[string]types.AttributeValue)

	item["UserID"] = &types.AttributeValueMemberS{Value: uuid.Must(uuid.NewRandom()).String()}
	item["email"] = &types.AttributeValueMemberS{Value: event.Email}
	item["password"] = &types.AttributeValueMemberS{Value: event.Password}
	item["username"] = &types.AttributeValueMemberS{Value: event.Username}
	item["name"] = &types.AttributeValueMemberS{Value: event.Name}
	item["timeTilExpire"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(time.Now().UTC().Add(time.Minute*15).Unix(), 10)}
	item["verificationCode"] = &types.AttributeValueMemberN{Value: produceRandomNDigits(5)}

	//put list of strings into correct format
	av, err := attributevalue.MarshalList(event.Schedule)

	if err != nil {
		return events.APIGatewayV2HTTPResponse{}, err
	}

	item["schedule"] = &types.AttributeValueMemberL{Value: av}

	_, err = client.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item:      item,
	})

	if err != nil {
		return events.APIGatewayV2HTTPResponse{}, err
	}
	return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusOK}, nil
}
