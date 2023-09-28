package main

import (
	"context"
	"errors"
	"math/rand"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

func GetUserIDfromEmail(email string) (string, error) {
	item := make(map[string]types.AttributeValue)

	item[":email"] = &types.AttributeValueMemberS{Value: email}
	QueryInput, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		ExpressionAttributeValues: item,
		IndexName:                 aws.String("email-index"),
		KeyConditionExpression:    aws.String("email = :email"),
		ProjectionExpression:      aws.String("UserID"),
	})

	if err != nil {
		return "", err
	}

	//no code returned
	if QueryInput.Count == 0 {
		return "", errors.New("no email found")
	}

	ret := make(map[string]interface{})
	attributevalue.UnmarshalMap(QueryInput.Items[0], &ret)
	userid := ret["UserID"].(string)

	return userid, nil
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
