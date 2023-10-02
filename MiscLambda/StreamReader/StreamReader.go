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
	BuildingLong float64 `json:"BuildingLong"`
	BuildingLat  float64 `json:"BuildingLat"`
}
type Event struct {
	EventID     string        `json:"EventID"`
	DateTime    string        `json:"dateTime"`
	Description string        `json:"description"`
	Host        string        `json:"host"`
	Location    EventLocation `json:"location"`
	Name        string        `json:"name"`
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

// UnmarshalStreamImage converts events.DynamoDBAttributeValue to struct
func UnmarshalStreamImage(attribute map[string]events.DynamoDBAttributeValue, out interface{}) error {

	dbAttrMap := make(map[string]types.AttributeValue)

	/*for k, v := range attribute {
		log.Println(k, v)

		var dbAttr types.AttributeValue

		bytes, marshalErr := v.MarshalJSON()
		if marshalErr != nil {
			return marshalErr
		}
		log.Println(string(bytes))
		err := json.Unmarshal(bytes, &dbAttr)
		if err != nil {
			log.Println(err)
		}
		log.Println(dbAttr, bytes)
		dbAttrMap[k] = dbAttr
	}*/

	log.Println(dbAttrMap)

	return attributevalue.UnmarshalMap(dbAttrMap, out)
}
func handler(ctx context.Context, event events.DynamoDBEvent) {
	log.Printf("in handler")
	log.Println(event.Records)
	// go through all the records
	for _, record := range event.Records {
		log.Println("In for loop")
		// if this was a remove record, that's what we're interested in
		if record.EventName == "REMOVE" {
			//oldImage := convertDynamoDBAttributes(record.Change.OldImage)
			oldImage := Event{}
			UnmarshalStreamImage(record.Change.OldImage, oldImage)
			log.Println(oldImage)
			// after unmarshaling the event, create an update input for the building table
			updateInput := &dynamodb.UpdateItemInput{
				TableName: aws.String(table),
				Key: map[string]types.AttributeValue{
					"BuildingLong": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", oldImage.Location.BuildingLong)},
					"BuildingLat":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", oldImage.Location.BuildingLat)},
				},
				UpdateExpression: aws.String("DELETE BuildingEvents :evtId"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":evtId": &types.AttributeValueMemberSS{Value: []string{oldImage.EventID}},
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
	// test := map[string]interface{}{
	// 	"CALL": true,
	// }

	// p, _ := dynamodbattribute.MarshalMap(test)

}
