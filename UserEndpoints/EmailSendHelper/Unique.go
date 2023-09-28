package main

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

var codes []string = []string{
	"verificationCode",
	"resetCode",
}

func GetEmailWithUserID(UserID string) (string, error) {
	item := make(map[string]types.AttributeValue)

	item["UserID"] = &types.AttributeValueMemberS{Value: UserID}

	TheInput, err := client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName:            aws.String(table),
		Key:                  item,
		ProjectionExpression: aws.String("email"),
	})

	//-----------------------------------------ERROR CHECKING-----------------------------------------
	//General error occured
	if err != nil {
		return "", err
	}
	//-----------------------------------------PACK RESULTS-----------------------------------------
	//get results in
	results := map[string]any{}
	attributevalue.UnmarshalMap(TheInput.Item, &results)

	email, emailOk := results["email"].(string)

	if !emailOk {
		return "", errors.New("no email field found")
	}

	return email, nil
}

func QueryCodes(email string, codeType int) (int, error) {
	item := make(map[string]types.AttributeValue)

	item[":email"] = &types.AttributeValueMemberS{Value: email}
	QueryInput, err := client.Query(context.Background(), &dynamodb.QueryInput{
		TableName:                 aws.String(table),
		ExpressionAttributeValues: item,
		IndexName:                 aws.String("email-index"),
		KeyConditionExpression:    aws.String("email = :email"),
		ProjectionExpression:      aws.String(codes[codeType%len(codes)]),
	})

	if err != nil {
		return -1, err
	}

	//no code returned
	if QueryInput.Count == 0 {
		return -1, errors.New("no code found")
	}

	ret := make(map[string]interface{})
	attributevalue.UnmarshalMap(QueryInput.Items[0], &ret)
	code := ret[codes[codeType%len(codes)]].(float64)

	return int(code), nil
}
