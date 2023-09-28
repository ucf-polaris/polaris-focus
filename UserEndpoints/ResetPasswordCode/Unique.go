package main

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

// is current time within the time frame of 'compare - minutes' and 'compare'
func CheckTime(minutes int64, compare int64) bool {
	first := compare - (minutes * 60)

	now := time.Now().UTC().Unix()

	return first <= now && now <= compare
}

// check if time is valid and if user is verified
func CheckIfValid(UserID string) error {
	item := make(map[string]types.AttributeValue)

	item["UserID"] = &types.AttributeValueMemberS{Value: UserID}
	GetOutput, err := client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName:            aws.String(table),
		Key:                  item,
		ProjectionExpression: aws.String("timeTilExpire, resetRequestExpireTime"),
	})

	if err != nil {
		return err
	}

	val, ok := GetOutput.Item["resetRequestExpireTime"]
	_, okCode := GetOutput.Item["timeTilExpire"]

	//is valid (timeTilExpire doesn't exist)
	if okCode {
		return errors.New("this is an non-validated user")
	}

	//has a resetRequestExpireTime
	if ok {
		var val_unmarsh float64

		err := attributevalue.Unmarshal(val, &val_unmarsh)
		if err != nil {
			return err
		}

		//check if timestamp, set for 15 minutes from when code was sent, is still valid
		if !CheckTime(15, int64(val_unmarsh)) {
			return errors.New("code is expired")
		}
	} else {
		return errors.New("no password reset request found")
	}

	return nil
}
