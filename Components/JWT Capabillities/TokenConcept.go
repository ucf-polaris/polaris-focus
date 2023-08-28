// A bunch of JWT functions for reference
package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	//Username string `json:"username"`
	jwt.RegisteredClaims
}

var table string
var sampleSecretKey = []byte("SecretYouShouldHide")

func main() {
	/*tkn, err := generateJWT()
	if err != nil {
		panic(err)
	}
	fmt.Println(tkn)*/
	err := verifyJWT("eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2OTMyMDc3MDB9.GFVFdm88P7FOWl4koO3Hrx2iIP3hYNpSUGGPo-n7FbEtvY1zLc8MMFxIb3FJcqYSMjOMTILlkBBCIXJrjy0VsA")
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Yes")
	}
	//lambda.Start(handler)
}

func verifyJWT(token string) error {
	/*if request.Header["Token"] != nil {

	}*/
	claims := &Claims{}
	tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return sampleSecretKey, nil
	})

	if err != nil {
		return err
	}
	if !tkn.Valid {
		return errors.New("invalid token")
	}
	return nil
}

func generateJWT() (string, error) {
	//The claims
	expirationTime := time.Now().Add(1 * time.Minute)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	tokenString, err := token.SignedString(sampleSecretKey)
	if err != nil {
		panic(err)
	}
	return tokenString, nil
}

// Help function to generate an IAM policy
func generatePolicy(principalId, effect, resource string) events.APIGatewayCustomAuthorizerResponse {
	authResponse := events.APIGatewayCustomAuthorizerResponse{PrincipalID: principalId}

	if effect != "" && resource != "" {
		authResponse.PolicyDocument = events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   effect,
					Resource: []string{resource},
				},
			},
		}
	}

	return authResponse
}

func handler(event events.APIGatewayCustomAuthorizerRequest) (events.APIGatewayCustomAuthorizerResponse, error) {
	token := event.AuthorizationToken
	switch strings.ToLower(token) {
	case "allow":
		return generatePolicy("user", "Allow", event.MethodArn), nil
	case "deny":
		return generatePolicy("user", "Deny", event.MethodArn), nil
	case "unauthorized":
		return events.APIGatewayCustomAuthorizerResponse{}, errors.New("Unauthorized") // Return a 401 Unauthorized response
	default:
		return events.APIGatewayCustomAuthorizerResponse{}, errors.New("Error: Invalid token")
	}
}
