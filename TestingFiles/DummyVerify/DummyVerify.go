package main

import (
	"errors"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func init() {

}
func main() {
	lambda.Start(handler)
}

// Help function to generate an IAM policy
func generatePolicy(stringKey, principalId, effect, resource string) events.APIGatewayCustomAuthorizerResponse {
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
		//CHECK IF I CAN PULL THIS FROM event.requestContext.authorizer.company_id in GetUser
		authResponse.Context = map[string]interface{}{
			"stringKey": stringKey,
		}
	}

	return authResponse
}

func handler(event events.APIGatewayCustomAuthorizerRequest) (events.APIGatewayCustomAuthorizerResponse, error) {
	token := event.AuthorizationToken
	switch strings.ToLower(token) {
	case "allow":
		return generatePolicy("", "user", "Allow", event.MethodArn), nil
	case "deny":
		return generatePolicy("", "user", "Deny", event.MethodArn), nil
	case "unauthorized":
		return generatePolicy("", "user", "Deny", event.MethodArn), errors.New("Unauthorized") // Return a 401 Unauthorized response
	default:
		return generatePolicy("no!", "user", "Deny", event.MethodArn), nil
	}
}
