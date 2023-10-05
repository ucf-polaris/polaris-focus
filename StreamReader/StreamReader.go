package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	dyn1 "github.com/aws/aws-sdk-go/service/dynamodb"
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

type DynamoEventChange struct {
	NewImage *dyn1.AttributeValue `json:"NewImage"`
	OldImage *dyn1.AttributeValue `json:"OldImage"`
	// ... more fields if needed: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_streams_GetRecords.html
}

type DynamoEventRecord struct {
	Change    DynamoEventChange `json:"dynamodb"`
	EventName string            `json:"eventName"`
	EventID   string            `json:"eventID"`
	// ... more fields if needed: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_streams_GetRecords.html
}

type DynamoDBEvent struct {
	Records []DynamoEventRecord `json:"records"`
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

func EvaluateType(e events.DynamoDBAttributeValue, name string) interface{} {
	switch e.DataType() {
	case events.DataTypeNumber:
		//if it's a number, get to float
		log.Println("NUMBER", name)
		val, err := e.Float()
		if err != nil {
			panic(err)
		}
		return val

	case events.DataTypeString:
		//if it's a string. Very Simple
		log.Println("STRING", name)
		return e.String()

	case events.DataTypeMap:
		//if it's a map, recursively call on itself
		log.Println("MAP", name)
		m := e.Map()
		map_create := make(map[string]interface{})

		//iterate through map of 'events.DynamoDBAttributes'
		for k, v := range m {
			new_val := EvaluateType(v, name)
			//check if there's type not accounted for
			if new_val == nil {
				panic(errors.New("returned nil in EvaluateType"))
			}
			map_create[k] = new_val
		}
		return map_create

	case events.DataTypeList:
		//if it's list, recursively call on itself (untested)
		log.Println("LIST", name)
		m := e.List()
		list_create := []interface{}{}

		//iterate through list of 'events.DynamoDBAttributes'
		for _, v := range m {
			new_val := EvaluateType(v, name)
			//check if there's type not accounted for
			if new_val == nil {
				panic(errors.New("returned nil in EvaluateType"))
			}
			list_create = append(list_create, new_val)
		}
		return list_create
	}

	return nil
}

func PackValues(record map[string]events.DynamoDBAttributeValue) map[string]interface{} {
	ret := make(map[string]interface{})

	for name, value := range record {
		//iterate through OldImage to get foriegn keys for deletion
		new_val := EvaluateType(value, name)
		//check if there's type not accounted for
		if new_val == nil {
			panic(errors.New("returned nil in EvaluateType"))
		}
		ret[name] = new_val
	}

	return ret
}

func operationOnList(record map[string]events.DynamoDBAttributeValue, mode int) {
	var evt Event
	m := PackValues(record)

	//get into struct from map interface
	js, _ := json.Marshal(m)
	err := json.Unmarshal(js, &evt)

	if err != nil {
		log.Println(err)
		return
	}

	log.Println(evt.Location.BuildingLong, evt.Location.BuildingLat)

	query := "DELETE"
	if mode == 1 {
		query = "ADD"
	}
	query += " BuildingEvents :evtId"

	// after unmarshaling the event, create an update input for the building table
	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"BuildingLong": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", evt.Location.BuildingLong)},
			"BuildingLat":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", evt.Location.BuildingLat)},
		},
		UpdateExpression: aws.String(query),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":evtId": &types.AttributeValueMemberSS{Value: []string{evt.EventID}},
		},
		ReturnValues: types.ReturnValueAllNew,
	}

	upd, err := client.UpdateItem(context.Background(), updateInput)
	if err != nil {
		log.Printf("Failed to update building, %v", err)
	}

	output := make(map[string]interface{})
	attributevalue.UnmarshalMap(upd.Attributes, &output)
	log.Println(output)
}

func handler(event events.DynamoDBEvent) {
	// go through all the records
	for _, record := range event.Records {
		// if this was a remove record, that's what we're interested in
		if record.EventName == "REMOVE" {
			operationOnList(record.Change.OldImage, 0)
			log.Println("Did Remove")
		} else if record.EventName == "INSERT" {
			operationOnList(record.Change.NewImage, 1)
			log.Println("Did Add")
		}
	}
}

func main() {
	lambda.Start(handler)
}
