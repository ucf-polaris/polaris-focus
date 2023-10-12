package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Building struct {
	BuildingLong         float64  `json:"BuildingLong"`
	BuildingLat          float64  `json:"BuildingLat"`
	BuildingDesc         string   `json:"BuildingDesc"`
	BuildingEvents       []string `json:"BuildingEvents,omitempty"`
	BuildingName         string   `json:"BuildingName"`
	BuildingAltitude     float64  `json:"BuildingAltitude,omitempty"`
	BuildingLocationType string   `json:"BuildingLocationType,omitempty"`
	BuildingAbbreviation string   `json:"BuildingAbbreviation,omitempty"`
	BuildingAllias 		 string   `json:"BuildingAllias,omitempty"`
	BuildingAddress      string   `json:"BuildingAddress,omitempty"`
	BuildingImage        string   `json:"BuildingImage,omitempty"`
}
type Payload struct {
	BuildingLong float64 `json:"BuildingLong"`
	BuildingLat  float64 `json:"BuildingLat"`
}

type Response struct {
	Building Building `json:"building"`
	Tokens   Tokens   `json:"tokens"`
}

type Tokens struct {
	Token        string `json:"token,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

var table string
var client *dynamodb.Client

func init() {
	//create session for dynamodb
	client, table = Helpers.ConstructDynamoHost()

	if table == "" {
		log.Fatal("missing environment variable TABLE_NAME")
	}
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	token, refreshToken, err := Helpers.GetTokens(request)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	var payload Payload
	err = json.Unmarshal([]byte(request.Body), &payload)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	bLong := payload.BuildingLong
	bLat := payload.BuildingLat

	// Fetch the building in the form of a go struct from the database
	building, err := getBuildingByLongLat(context.Background(), bLong, bLat)
	// If an error came up, early exit and return the error
	if err != nil {
		return Helpers.ResponseGeneration(fmt.Sprintf("fetching building data: %v", err), http.StatusBadRequest)
	}

	// If the building didn't end up existing, return that information to the caller
	if building == nil {
		return Helpers.ResponseGeneration("Building not found in table", http.StatusBadRequest)
	}

	tokens := Tokens{
		Token:        token,
		RefreshToken: refreshToken,
	}

	ret := Response{
		Building: *building,
		Tokens:   tokens,
	}

	// Convert the building go struct to a json for return
	buildingJSON, err := json.Marshal(ret)
	// If marshaling failed, early exit
	if err != nil {
		return Helpers.ResponseGeneration("when marshaling data", http.StatusBadRequest)
	}

	// Return the building info in the form of a stringified JSON
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(buildingJSON),
		Headers:    map[string]string{"content-type": "application/json"},
	}, nil
}

func getBuildingByLongLat(ctx context.Context, BuildingLong float64, BuildingLat float64) (*Building, error) {
	// Construct the get item input given the long and lat provided
	inp := &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"BuildingLong": &types.AttributeValueMemberN{Value: strconv.FormatFloat(BuildingLong, 'f', -1, 64)},
			"BuildingLat":  &types.AttributeValueMemberN{Value: strconv.FormatFloat(BuildingLat, 'f', -1, 64)},
		},
	}

	// Try to query dynamodb with this get item
	output, err := client.GetItem(ctx, inp)

	// Return the error if it fails
	if err != nil {
		return nil, err
	}

	// Return nil if the item didn't end up existing
	if output.Item == nil {
		return nil, nil
	}
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
