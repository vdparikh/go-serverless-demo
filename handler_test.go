package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/context"

	"github.com/stretchr/testify/assert"
)

func TestAuth(t *testing.T) {
	var jsonStr = []byte(`{"username":"user", "password", "pass"}`)
	r, _ := http.NewRequest("POST", "/auth", bytes.NewBuffer(jsonStr))
	w := httptest.NewRecorder()

	//Hack to try to fake gorilla/mux vars
	vars := map[string]string{
		"mystring": "abcd",
	}
	context.Set(r, 0, vars)

	login(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	log.Println(string(w.Body.Bytes()))

}

func TestGetUsers(t *testing.T) {
	r, _ := http.NewRequest("GET", "/users/vishal/tasks", nil)
	r.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJBdWRpZW5jZSIsImV4cCI6MTUxMTEzNjI2NCwiaWF0IjoxNTExMTM2MjA0LCJpc3MiOiJJc3N1ZXIiLCJqdGkiOiI4ZDA1ZWNiZi04MTY4LTRiNWUtODAxMy1mMDljMWMzNTBiZTMiLCJuYmYiOjIsInNjb3BlcyI6ImFwaTphY2Nlc3MiLCJzdWIiOiIifQ.bzPqZaaQXsW7qi5EYdi0UGC0sWQ1PeUWOEN1y2c2ShQ")
	w := httptest.NewRecorder()

	//Hack to try to fake gorilla/mux vars
	vars := map[string]string{
		"userId": "vishalparikh",
	}
	context.Set(r, 0, vars)

	getTasks(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	// assert.Equal(t, []byte("abcd"), w.Body.Bytes())
	log.Println(string(w.Body.Bytes()))

}
