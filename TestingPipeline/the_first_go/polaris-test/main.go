package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	ErrKeyNotFound    = fmt.Errorf("key not found")
	ErrTokenNotFound  = fmt.Errorf("token not found")
	ErrRecordNotFound = fmt.Errorf("record not found")
)

var client *dynamodb.Client

func constructDynamoHost() *dynamodb.Client {
	var err error
	var cfg aws.Config
	if isLambdaLocal() {
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
	} else {
		cfg, err = config.LoadDefaultConfig(context.Background())
		if err != nil {
			panic(err)
		}
	}

	return dynamodb.NewFromConfig(cfg)
}

func tokenExtraction(request events.APIGatewayProxyRequest) (string, string, error) {
	var token string
	var rfsTkn string
	if request.RequestContext.Authorizer != nil {
		contextString := request.RequestContext.Authorizer["stringKey"].(string)

		ctxt := make(map[string]interface{})
		err := json.Unmarshal([]byte(contextString), &ctxt)
		if err != nil {
			return "", "", err
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

func init() {
	client = constructDynamoHost()
}

func isLambdaLocal() bool {
	_, err := os.Stat("./main_test.go")
	return err == nil
}

func unpackRequest(body string) map[string]interface{} {
	if body == "" {
		return nil
	}

	search := make(map[string]interface{})
	err := json.Unmarshal([]byte(body), &search)

	if err != nil {
		panic(err)
	}

	return search
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	token, refresh, err := tokenExtraction(request)
	if err != nil {
		return events.APIGatewayProxyResponse{
			Body:       "",
			StatusCode: 200,
		}, ErrTokenNotFound
	}

	req := unpackRequest(request.Body)

	id, ok := req["UserID"].(string)
	id2, ok2 := req["id"].(float64)

	if !ok || !ok2 {
		return events.APIGatewayProxyResponse{
			Body:       "",
			StatusCode: 200,
		}, ErrKeyNotFound
	}

	item := make(map[string]types.AttributeValue)
	item["UserID"] = &types.AttributeValueMemberS{Value: id}
	item["id"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(int64(id2), 10)}

	TheInput, err := client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String("THENEWTABLE"),
		Key:       item,
	})

	if err != nil {
		return events.APIGatewayProxyResponse{
			Body:       err.Error(),
			StatusCode: 200,
		}, err
	}

	if len(TheInput.Item) == 0 {
		return events.APIGatewayProxyResponse{
			Body:       "",
			StatusCode: 200,
		}, ErrRecordNotFound
	}

	ret := make(map[string]interface{})
	attributevalue.UnmarshalMap(TheInput.Item, &ret)
	ret["token"] = token
	ret["refreshToken"] = refresh

	js, err := json.Marshal(ret)
	if err != nil {
		return events.APIGatewayProxyResponse{
			Body:       "",
			StatusCode: 200,
		}, err
	}

	return events.APIGatewayProxyResponse{
		Body:       string(js),
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(handler)
}
