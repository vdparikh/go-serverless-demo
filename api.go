package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// AuthTokenClaim ...
// This is the cliam object which gets parsed from the authorization header
type AuthTokenClaim struct {
	jwt.MapClaims
	User
}

// TokenResponse ...
type TokenResponse struct {
	TokenType string `json:"token_type"`
	Token     string `json:"access_token"`
	ExpiresIn int64  `json:"expires_in"`
}

// ErrorMsg ...
type ErrorMsg struct {
	Message     string `json:"message"`
	Description string `json:"description"`
}

// User ...
type User struct {
	Username string `dynamo:"username" json:"username"`
	Name     string `dynamo:"name" json:"name"`
	Password string `dynamo:"password" json:"password"`
}

// UserTask ...
type UserTask struct {
	ID          string `dynamo:"id" json:"id"`
	UserID      string `dynamo:"userId" json:"userId"`
	Title       string `dynamo:"title" json:"title"`
	Description string `dynamo:"description" json:"description"`
	Icon        string `dynamo:"icon" json:"icon"`
}

// Utility function to store the password as MD5 hash
func getMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Utility function to response with JSON and setting header
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	// Even if you enable CORS in API Gateway, the integration response aka the API response from Lambda
	// needs to return the headers for it to wokr
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

	// Set response status code
	w.WriteHeader(code)

	// Encode and reply
	json.NewEncoder(w).Encode(payload)
}

// Generate JWT Token
func generateToken(username string, expiryMinutes time.Duration) (string, int64, error) {
	expiresAt := time.Now().Add(time.Minute * expiryMinutes).Unix()
	token := jwt.New(jwt.SigningMethodHS256)

	token.Claims = jwt.MapClaims{
		"iss":    "Issuer",            // who creates the token and signs it
		"aud":    "Audience",          // to whom the token is intended to be sent
		"exp":    expiresAt,           // time when the token will expire (10 minutes from now)
		"jti":    uuid.New().String(), // a unique identifier for the token
		"iat":    time.Now().Unix(),   // when the token was issued/created (now)
		"nbf":    2,                   // time before which the token is not yet valid (2 minutes ago)
		"sub":    username,            // the subject/principal is whom the token is about
		"scopes": "api:access",        // token scope - not a standard claim
	}

	// This is temporary hardcoded value
	// Replace it with something sensible like KMS
	tokenSecret := []byte("secret")
	tokenString, error := token.SignedString(tokenSecret)

	return tokenString, expiresAt, error
}

func login(w http.ResponseWriter, r *http.Request) {

	requestID := r.Header.Get("aws-request-id")

	// Map Input to User
	var user User
	b, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(b, &user)

	// Basic Validation
	if user.Username == "" || user.Password == "" {
		log.WithFields(log.Fields{
			"request-id": requestID,
			"error":      "Missing username or Password",
		}).Error("RequestFailed")
		respondWithJSON(w, http.StatusBadRequest, ErrorMsg{Message: "Missing Username and/or Password", Description: "Invalid Values"})
		return
	}

	// Query DB to select the user
	// In real world you will md5 the password first and then query as you will have it hashed prior to storing
	var dbuser User
	user.Password = getMD5Hash(user.Password)
	err := userTable.Get("username", user.Username).Consistent(true).Filter("password = ?", user.Password).One(&dbuser)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, ErrorMsg{Message: "Invalid username or password", Description: err.Error()})
		return
	}

	// Genearte a token which expires after 10 minutes and subject name as the username
	// This will be used for validation in the authorizer and if the incoming request userId in /users/<userId>/tasks
	// does not match the subject then the authorizer will reject the request and will never be executed.
	tokenString, expiresAt, error := generateToken(dbuser.Username, 10)
	if error != nil {
		respondWithJSON(w, http.StatusInternalServerError, ErrorMsg{Message: "Failed Creating Token", Description: error.Error()})
		return
	}

	// Generate Response Object
	var authResponse = TokenResponse{
		Token:     tokenString,
		TokenType: "Bearer",
		ExpiresIn: expiresAt,
	}

	respondWithJSON(w, http.StatusOK, authResponse)
}

func register(w http.ResponseWriter, r *http.Request) {

	// Input Validation
	var user User
	b, _ := ioutil.ReadAll(r.Body)
	err := json.Unmarshal(b, &user)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, ErrorMsg{Message: "Failed Reading Input", Description: err.Error()})
		return
	}

	// Add user to the database
	// In real world you will md5 the password before storing
	user.Password = getMD5Hash(user.Password)
	err = userTable.Put(user).Run()
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, ErrorMsg{Message: "Failed Creating User", Description: err.Error()})
		return
	}

	// Reply back with Status Created
	respondWithJSON(w, http.StatusCreated, user.Username)
}

// GET User Tasks
func getTasks(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	// Query DB for all tasks assocaited with the userId coming in the URI /users/<userId>/tasks
	// The authorizer will have already perform validation on request validity and so no need for extra checks here
	var results []UserTask
	err := taskTable.Scan().Filter("'userId' = ?", vars["userId"]).All(&results)

	// If error getting data then return error back
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, ErrorMsg{Message: "Failed Getting Tasks", Description: err.Error()})
		return
	}

	// Return all tasks
	respondWithJSON(w, http.StatusOK, results)
}

// POST API Example
func addTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	// Input Validation
	var task UserTask
	b, _ := ioutil.ReadAll(r.Body)
	err := json.Unmarshal(b, &task)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, ErrorMsg{Message: "Failed Reading Input", Description: err.Error()})
		return
	}

	// Associate task with the userId coming in the URI /users/<userId>/tasks
	// The authorizer will have already perform validation on request validity and so no need for extra checks here
	task.UserID = vars["userId"]

	// Generate unique ID for the Task
	task.ID = uuid.New().String()

	err = taskTable.Put(task).Run()
	// If error creating task then return error back
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, ErrorMsg{Message: "Failed Creating Task", Description: err.Error()})
		return
	}

	// Return Success
	respondWithJSON(w, http.StatusCreated, task)
}

// POST API Example
func deleteTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	// Delete Task
	// TODO: Add validation and filter/scan on the userId. The Task can only be associated with the
	// user who is calling the API. We don't need to do user authorization howevrer
	err := taskTable.Delete("id", vars["taskId"]).Run()

	// If error deleting task then return error back
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, ErrorMsg{Message: "Failed Deleting Task", Description: err.Error()})
		return
	}

	// Return Success
	respondWithJSON(w, http.StatusOK, nil)
}
