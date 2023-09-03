package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type User struct {
	UserID   int      `json:"UserID"`
	Email    string   `json:"email"`
	Password string   `json:"password"`
	Schedule []string `json:"schedule"`
	Username string   `json:"username"`
}

func main() {
	fmt.Println("hi" + "no")
	cfg, err := config.LoadDefaultConfig(context.TODO(), func(o *config.LoadOptions) error {
		o.Region = "us-east-2"
		return nil
	})
	if err != nil {
		panic(err)
	}
	// Using the Config value, create the DynamoDB client
	svc := dynamodb.NewFromConfig(cfg)

	getItemInput := &dynamodb.GetItemInput{
		Key: map[string]types.AttributeValue{
			"UserID": &types.AttributeValueMemberN{Value: "0"},
		},
		TableName: aws.String("Users"),
	}

	getItemResponse, err := svc.GetItem(context.TODO(), getItemInput)

	newUser := User{}
	fmt.Println(newUser)
	attributevalue.UnmarshalMap(getItemResponse.Item, &newUser)
	fmt.Println(newUser)
}
