package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/eawsy/aws-lambda-go-core/service/lambda/runtime"
	"github.com/eawsy/aws-lambda-go-net/service/lambda/runtime/net/apigatewayproxy"
)

//AuthStruct ...
type AuthStruct struct {
	AuthorizationToken string `json:"authorizationToken"`
	MethodArn          string `json:"methodArn"`
	Type               string `json:"type"`
}

// StatementStruct ...
type StatementStruct struct {
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource []string `json:"Resource"`
}

// AuthorizerResponse ...
type AuthorizerResponse struct {
	PrincipalID    string `json:"principalId"`
	PolicyDocument struct {
		Version   string            `json:"Version"`
		Statement []StatementStruct `json:"Statement"`
	} `json:"policyDocument"`
}

// VerifyHandle ... Verify OAUTH token and generate access policy
// This handler is not exposed as an API but is calling directly by the custom authorizer
var VerifyHandle apigatewayproxy.Handler

// generatePolicy
func generatePolicy(effect string, eventMap map[string]interface{}, user string) AuthorizerResponse {

	resource := eventMap["methodArn"].(string)

	var authResponse AuthorizerResponse
	authResponse.PrincipalID = user
	authResponse.PolicyDocument.Version = "2012-10-17" // default version
	authResponse.PolicyDocument.Statement = append(
		authResponse.PolicyDocument.Statement,
		StatementStruct{effect, []string{"execute-api:Invoke"}, []string{resource}},
	)

	// authResponse.Headers = map[string]string{"Access-Control-Allow-Origin": "*", "Access-Control-Allow-Credentials": "true"}
	// You cannot explicitly set headers in the authorizer and has to be manually done from Console via Gateway Resources page
	return authResponse
}

// VerifyHandler ...
// Function JWT Token and generates access policy for API Gateway
func VerifyHandler(evt json.RawMessage, ctx *runtime.Context) (interface{}, error) {

	// Input Verification
	// The Input to authorizer should be as below
	// {
	// 	"type":"TOKEN",
	// 	"authorizationToken":"<caller-supplied-token>",
	// 	"methodArn":"arn:aws:execute-api:<regionId>:<accountId>:<apiId>/<stage>/<method>/<resourcePath>"
	// }
	var e interface{}
	_ = json.Unmarshal(evt, &e)
	eventMap := e.(map[string]interface{})

	authStruct := AuthStruct{}
	err := json.Unmarshal(evt, &authStruct)
	if err != nil {
		return nil, errors.New(`Error: Invalid token`)
	}

	authorizationHeader := authStruct.AuthorizationToken
	if authorizationHeader != "" {
		bearerToken := strings.Split(authorizationHeader, " ")
		// The token is normally as "bearer xxxx" and make sure length is 2
		if len(bearerToken) == 2 {
			token, error := jwt.Parse(bearerToken[1], func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("There was an error")
				}
				// As always use KMS or something better for secret management
				return []byte("secret"), nil
			})

			// If token is Expired and/or invalid
			if error != nil {
				if error.Error() == "Token is expired" {
					return nil, errors.New(`Unauthorized`)
				}
				return nil, errors.New(`Unauthorized`)
			}

			if token.Valid {
				claims := token.Claims.(jwt.MapClaims)
				tmp := strings.Split(authStruct.MethodArn, ":")
				// Now this truly a hack for now to get the userId from the URI
				// TODO: Make it better
				if len(tmp) > 5 {
					apiGatewayArnTmp := strings.Split(tmp[5], "/")
					if len(apiGatewayArnTmp) > 4 {
						subject := claims["sub"].(string)
						if apiGatewayArnTmp[4] == subject {
							return generatePolicy("Allow", eventMap, subject), nil
						}
					}
				}
			}
		}
	}

	return nil, errors.New(`Unauthorized`)

}
