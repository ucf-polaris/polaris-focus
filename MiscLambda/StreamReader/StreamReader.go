package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

type EventLocation struct {
	BuildingLong    float64    `json:"BuildingLong"`
	BuildingLat     float64    `json:"BuildingLat"`
}
type Event struct {
	EventID         string          `json:"EventID"`
	DateTime        string          `json:"dateTime"`
	Description     string          `json:"description"`
	Host            string          `json:"host"`
	Location        EventLocation   `json:"location"`
	Name            string          `json:"name"`
}

var client *dynamodb.Client
var table string

func init() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	table = "Buildings"
	if err != nil {
		log.Fatalf("Failed to load config, %v", err)
	}
	client = dynamodb.NewFromConfig(cfg)
}

func convertDynamoDBAttributes(old map[string]events.DynamoDBAttributeValue) map[string]types.AttributeValue {
	ret := make(map[string]types.AttributeValue)
	for k, v := range old {
		if v.DataType() == events.DataTypeString {
			ret[k] = &types.AttributeValueMemberS{Value: v.String()}
		} else if v.DataType() == events.DataTypeNumber {
			ret[k] = &types.AttributeValueMemberN{Value: v.Number()}
		}
	}
	return ret
}

func handler(ctx context.Context, event events.DynamoDBEvent) {
	// go through all the records
	for _, record := range event.Records {
		// if this was a remove record, that's what we're interested in
		if record.EventName == "REMOVE" {
			// grab the old image (the object that just got deleted)
			oldImage := convertDynamoDBAttributes(record.Change.OldImage)
			// initialize an event to unmarshal to
			var evt Event
			err := attributevalue.UnmarshalMap(oldImage, &evt)
			if err != nil {
				log.Printf("Failed to unmarshal record, %v", err)
				continue
			}
			// after unmarshaling the event, create an update input for the building table
			updateInput := &dynamodb.UpdateItemInput{
				TableName: aws.String(table),
				Key: map[string]types.AttributeValue{
					"BuildingLong": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", evt.Location.BuildingLong)},
					"BuildingLat": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", evt.Location.BuildingLat)},
				},
				UpdateExpression: aws.String("DELETE BuildingEvents :evtId"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":evtId": &types.AttributeValueMemberSS{Value: []string{evt.EventID}},
				},
			}

			if _, err := client.UpdateItem(ctx, updateInput); err != nil {
				log.Printf("Failed to update building, %v", err)
			}
		}
	}
}

func main() {
	lambda.Start(handler)
}
