package main

import (
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/eawsy/aws-lambda-go-net/service/lambda/runtime/net"
	"github.com/eawsy/aws-lambda-go-net/service/lambda/runtime/net/apigatewayproxy"
	"github.com/gorilla/mux"
	"github.com/guregu/dynamo"
)

// Handle ... AWS Handler called by Lambda
var Handle apigatewayproxy.Handler

// Declare the 2 DymnamoDB tables we are going to use for storing users and their tasks
var taskTable dynamo.Table
var userTable dynamo.Table

func init() {

	// DynamoDB setup
	db := dynamo.New(session.New(), &aws.Config{Region: aws.String("us-east-1")})
	taskTable = db.Table("SERVERLESS_TASKS")
	userTable = db.Table("SERVERLESS_USERS")

	// Handler setup
	ln := net.Listen()
	Handle = apigatewayproxy.New(ln, []string{"image/png"}).Handle

	// MUX routing for the API calls
	// refer api.go for more details on the call
	r := mux.NewRouter()

	// User Authentication and Registration API
	r.Path("/auth").Methods("POST").HandlerFunc(login)
	r.Path("/register").Methods("POST").HandlerFunc(register)

	// Tasks API
	r.Path("/users/{userId}/tasks").Methods("GET").HandlerFunc(getTasks)
	r.Path("/users/{userId}/tasks").Methods("POST").HandlerFunc(addTask)
	r.Path("/users/{userId}/tasks/{taskId}").Methods("GET").HandlerFunc(getTasks)
	r.Path("/users/{userId}/tasks/{taskId}").Methods("DELETE").HandlerFunc(deleteTask)

	go http.Serve(ln, LoggingMiddleware(r))
}

func main() {
	// Do Nothing
}
