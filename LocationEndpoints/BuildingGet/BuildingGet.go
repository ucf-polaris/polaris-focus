package main

import (
	"context"
	"log"
	"net/http"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/aws"
)

type Building struct {
	BuildingLong    float64    		`json:"BuildingLong"`
	BuildingLat     float64    		`json:"BuildingLat"`
	BuildingDesc    string          `json:"BuildingDesc"`
	BuildingEvents  []string        `json:"BuildingEvents"`
	BuildingName    string   		`json:"BuildingName"`
}
type Payload struct {
	BuildingLong	float64		`json:"BuildingLong"`
	BuildingLat		float64		`json:"BuildingLat"`
}

var table string
var db *dynamodb.Client

func init() {
	table = "Buildings"
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load config, %v", err)
	}
	db = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var payload Payload

    err := json.Unmarshal([]byte(request.Body), &payload)
    if err != nil {
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusBadRequest,
            Body:       "Invalid input format",
        }, nil
    }

    bLong := payload.BuildingLong
    bLat := payload.BuildingLat

	// Fetch the building in the form of a go struct from the database
	building, err := getBuildingByLongLat(ctx, bLong, bLat)
	// If an error came up, early exit and return the error
	if err != nil {
		log.Printf("Error: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:		fmt.Sprintf("Error fetching building data: %v", err),
		}, err
	}

	// If the building didn't end up existing, return that information to the caller
	if building == nil {
		return events.APIGatewayProxyResponse {
			StatusCode: http.StatusBadRequest,
			Body: 		"Building not found in table",
		}, nil
	}

	// Convert the building go struct to a json for return
	buildingJSON, err := json.Marshal(building)
	// If marshaling failed, early exit
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body: 		"Error converting building data to JSON",
		}, err
	}

	// Return the building info in the form of a stringified JSON
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body: 		string(buildingJSON),
	}, nil
}

func getBuildingByLongLat(ctx context.Context, BuildingLong float64, BuildingLat float64) (*Building, error) {
	// Construct the get item input given the long and lat provided
	inp := &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"BuildingLong": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", BuildingLong)},
			"BuildingLat": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", BuildingLat)},
		},
	}

	// Try to query dynamodb with this get item
	output, err := db.GetItem(ctx, inp)

	// Return the error if it fails
	if err != nil {
		return nil, err
	}

	// Return nil if the item didn't end up existing
	if output.Item == nil {
		return nil, nil
	}
	log.Printf("Dynamo return: %v", output.Item)
	// construct the go struct from dynamo's item
	building := &Building{}
	err = attributevalue.UnmarshalMap(output.Item, building)
	if err != nil { // if this failed, early exit
		return nil, err
	}
	
	// yay!
	return building, nil
}

func main() {
	lambda.Start(handler)
}