// An early version of the token verification (migrate functions to handler later)
package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	//Username string `json:"username"`
	jwt.RegisteredClaims
}

type JWTPackage struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

var secretKey []byte

func init() {
	key := os.Getenv("SECRET_KEY")

	if key == "" {
		log.Fatal("missing environment variable SECRET_KEY")
	}
	secretKey = []byte(key)
}

func main() {
	lambda.Start(handler)
}

func verifyJWT(token string) error {
	claims := &Claims{}
	tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})

	if err != nil {
		log.Println(err)
		return err
	}
	if !tkn.Valid {
		return errors.New("invalid token")
	}
	return nil
}

func generateJWT(timeTil int) (string, error) {
	//The claims
	expirationTime := time.Now().UTC().Add(time.Duration(timeTil) * time.Minute)

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		panic(err)
	}
	return tokenString, nil
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
	var token string
	var ret_pack JWTPackage
	body := event.AuthorizationToken
	if event.AuthorizationToken == "" {
		return generatePolicy("AuthorizationToken Header Empty", "user", "Deny", event.MethodArn), nil
	}

	pack := JWTPackage{}

	//check the regular token
	json.Unmarshal([]byte(body), &pack)
	token = pack.Token
	refreshToken := pack.RefreshToken

	err := verifyJWT(token)
	//if and only if the token is expired due to a bad date, make new key
	if err != nil {

		//check if error type is Expired
		if strings.Contains(err.Error(), jwt.ErrTokenExpired.Error()) {

			//check if refresh token exists, if not return expired error (of token)
			if refreshToken == "" {
				return generatePolicy(err.Error(), "user", "Deny", event.MethodArn), nil
			}

			//check if refresh token is valid
			err := verifyJWT(refreshToken)
			if err != nil {
				return generatePolicy(err.Error(), "user", "Deny", event.MethodArn), nil
			}

			//generate new JWT token (if refresh token is valid)
			newTok, err := generateJWT(15)
			//if error comes up, throw exception
			if err != nil {
				return generatePolicy(err.Error(), "user", "Deny", event.MethodArn), nil
			}

			//return everything and generate policy (as long as refresh is okay, you're okay), security risk?
			ret_pack := JWTPackage{Token: newTok, RefreshToken: refreshToken}
			ret, err := json.Marshal(ret_pack)
			if err != nil {
				return generatePolicy(err.Error(), "user", "Deny", event.MethodArn), nil
			}

			return generatePolicy(string(ret), "user", "Allow", event.MethodArn), nil

		} else {
			return generatePolicy(err.Error(), "user", "Deny", event.MethodArn), nil
		}

	}

	//give keys back
	ret, err := json.Marshal(JWTPackage{Token: token, RefreshToken: refreshToken})
	if err != nil {
		return generatePolicy(err.Error(), "user", "Deny", event.MethodArn), err
	}

	//check the refresh token here and implement logic to get token
	return generatePolicy(string(ret), "user", "Allow", event.MethodArn), nil

	/*switch strings.ToLower(token) {
	case "allow":
		return generatePolicy("user", "Allow", event.MethodArn), nil
	case "deny":
		return generatePolicy("user", "Deny", event.MethodArn), nil
	case "unauthorized":
		return events.APIGatewayCustomAuthorizerResponse{}, errors.New("Unauthorized") // Return a 401 Unauthorized response
	default:
		return events.APIGatewayCustomAuthorizerResponse{}, errors.New("Error: Invalid token")
	}*/
}