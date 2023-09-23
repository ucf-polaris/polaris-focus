package main

import (
	"polaris-api/pkg/Helpers"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// make a TTL
func makeTTL(item map[string]types.AttributeValue, search map[string]interface{}, expire int) error {
	date, _ := search["dateTime"].(string)
	thetime, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return err
	}

	var timeVal string
	if expire == -2 {
		timeVal = strconv.FormatInt(thetime.UTC().Add(time.Hour*24).Unix(), 10)
	} else if expire <= 0 {
		timeVal = "0"
	} else {
		timeVal = strconv.FormatInt(thetime.UTC().Add(time.Hour*time.Duration(expire)).Unix(), 10)
	}

	//make sure dates aren't older than the current day (or by 5 years)
	item["timeTilExpire"] = &types.AttributeValueMemberN{Value: timeVal}

	return nil
}

func produceUUID() string {
	//allows unit testing to be consistent
	if Helpers.IsLambdaLocal() {
		return "0"
	}
	return uuid.Must(uuid.NewRandom()).String()
}
