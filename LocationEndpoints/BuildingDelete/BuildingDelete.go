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
	"github.com/aws/aws-sdk-go-v2/aws"
)

type Payload struct {
	BuildingLong	float64		`json:"BuildingLong"`
	BuildingLat		float64		`json:"BuildingLat"`
}

var table string
var client *dynamodb.Client

func init() {
	table = "Buildings"
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load config, %v", err)
	}
	client = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// iniitalize the longlat payload structure
	var payload Payload
	// unmarshal the input and error check if something went wrong
    err := json.Unmarshal([]byte(request.Body), &payload)
    if err != nil {
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusBadRequest,
            Body:       "Invalid input format",
        }, nil
    }

	// extract long and lat from the payload
	blat := payload.BuildingLat
	blong := payload.BuildingLong
	// if the blong and blat were not found, the float64 becomes 0.0 0.0
	// these coordinates are not in the scope of ucf, so it isn't a problem to use
	if blat == 0.0 || blong == 0.0 {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Building long or lat missing",
		}, nil
	}

	// Construct delete input
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"BuildingLong": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", blong)},
			"BuildingLat": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", blat)},
		},
		ConditionExpression: aws.String("attribute_exists(BuildingLong) AND attribute_exists(BuildingLat)"),
	}

	// delete building and error check it
	_, err = client.DeleteItem(ctx, input)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Error when deleting building from table, building may not exist"),
		}, nil
	}

	// Finally, return that the item was successfully deleted as expected.
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       "Building deleted successfully",
	}, nil
}

func main() {
	lambda.Start(handler)
}