package main

import (
	"encoding/json"
	"errors"
	"polaris-api/pkg/Helpers"
)

func ConstructVerified(queryResponse User, password string) (string, error) {
	//store and hide the password
	if queryResponse.Password == "" {
		return "", errors.New("query returned no password field")
	}

	//checking the password, if nothing return error
	if queryResponse.Password != password {
		return "", errors.New("invalid email/password")
	}

	queryResponse.Password = ""

	//-----------------------------------------TOKEN-----------------------------------------
	ret := make(map[string]interface{})
	tokens := make(map[string]interface{})
	//make and return token and refresh token
	tkn, err := Helpers.CreateToken(lambdaClient, 15, "", 0)
	if err != nil {
		return "", err
	}

	rfs, err := Helpers.CreateToken(lambdaClient, 1440, "", 1)
	if err != nil {
		return "", err
	}

	tokens["token"] = tkn
	tokens["refreshToken"] = rfs

	ret["User"] = queryResponse
	ret["tokens"] = tokens

	//package the results
	js, err := json.Marshal(ret)
	if err != nil {
		return "", err
	}

	return string(js), nil
}

func ConstructNonVerified(queryResponse User) (string, error) {
	if queryResponse.UserID == "" {
		return "", errors.New("ID field not found")
	}

	if queryResponse.Email == "" {
		return "", errors.New("email field not found")
	}

	user := make(map[string]interface{})
	user["UserID"] = queryResponse.UserID
	user["email"] = queryResponse.Email

	ret := make(map[string]interface{})
	//construct token with userID embedded
	regtkn, err := Helpers.CreateToken(lambdaClient, 15, queryResponse.UserID, 2)
	if err != nil {
		return "", err
	}

	ret["tokens"] = map[string]string{
		"token": regtkn,
	}
	ret["User"] = user

	js, err := json.Marshal(ret)
	if err != nil {
		return "", err
	}

	return string(js), nil
}
