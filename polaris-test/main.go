package main

import (
	"context"
	"encoding/json"
<<<<<<< HEAD
	"errors"
	"log"
	"net/http"
	"polaris-api/pkg/Helpers"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaCall "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	UserID string `json:"userID,omitempty"`
	jwt.RegisteredClaims
}

type CodeQuery struct {
	UserID string `json:"UserID"`
	Code   string `json:"code,omitempty"`
=======
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
>>>>>>> 8a9028944cb86dfa66684ebfbb4f8b8125458b0e
}

var table string
var client *dynamodb.Client
var lambdaClient *lambdaCall.Lambda

func init() {
	//dynamo stuff
	client, table = Helpers.ConstructDynamoHost()

	//set up lambda client
	//create session for lambda
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	lambdaClient = lambdaCall.New(sess, &aws.Config{Region: aws.String("us-east-2")})

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
<<<<<<< HEAD

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

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//-----------------------------------------EXTRACT FIELDS-----------------------------------------
	var contextString string
	query := CodeQuery{}
	ctxt := make(map[string]interface{})

	err := json.Unmarshal([]byte(request.Body), &query)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusBadRequest)
	}

	if request.RequestContext.Authorizer != nil {
		contextString = request.RequestContext.Authorizer["stringKey"].(string)
		log.Println(contextString)
	}

	json.Unmarshal([]byte(contextString), &ctxt)

	//look for email from JWT first, if not there look in passed in body
	if val, ok := ctxt["UserID"].(string); ok && query.UserID == "" {
		query.UserID = val
	}

	if query.Code == "" {
		return Helpers.ResponseGeneration("code field not set", http.StatusOK)
	}

	codeStr := query.Code
	//-----------------------------------------VAIDATE USER-----------------------------------------
	if err := CheckIfValid(query.UserID); err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}
	//-----------------------------------------THE UPDATE CALL-----------------------------------------
	//pass changes into update
	item := make(map[string]types.AttributeValue)
	item[":code"] = &types.AttributeValueMemberS{Value: codeStr}

	//who we're trying to find
	key := make(map[string]types.AttributeValue)
	key["UserID"] = &types.AttributeValueMemberS{Value: query.UserID}

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeValues: item,
		TableName:                 aws.String(table),
		Key:                       key,
		UpdateExpression:          aws.String("remove resetCode, resetRequestExpireTime"),
		ConditionExpression:       aws.String("resetCode = :code"),
	}

	_, err = client.UpdateItem(context.Background(), input)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	//-----------------------------------------RESULTS PROCESSING-----------------------------------------
	map_output := make(map[string]interface{})
	ret := make(map[string]interface{})
	ret["success"] = true
	//-----------------------------------------CREATE TOKENS-----------------------------------------
	map_output["token"], err = Helpers.CreateToken(lambdaClient, 15, "", 0)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	map_output["refreshToken"], err = Helpers.CreateToken(lambdaClient, 1440, "", 1)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	ret["tokens"] = map_output

	js, err := json.Marshal(ret)
	if err != nil {
		return Helpers.ResponseGeneration(err.Error(), http.StatusOK)
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(js), Headers: map[string]string{"content-type": "application/json"}}, nil
}
=======
>>>>>>> 8a9028944cb86dfa66684ebfbb4f8b8125458b0e
