package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type response struct {
	Error    string      `json:"error"`
	Response interface{} `json:"response,omitempty"`
}

func (r *response) String() string {
	data, _ := json.Marshal(r)
	return string(data)
}

func (srv *MyApi) ProfileHandler(w http.ResponseWriter, r *http.Request) {
	var param *ProfileParams = &ProfileParams{}

	if r.Method != "POST" && r.Method != "GET" {
		http.Error(w, "Bad method", http.StatusNotAcceptable)
		return
	}
	var urlVal url.Values
	if r.Method == "GET" {
		urlVal = r.URL.Query()
	} else {
		r.ParseForm()
		urlVal = r.Form
	}

	err := param.Unmarshall(urlVal)
	if err != nil {
		res := response{Error: err.Error()}
		http.Error(w, res.String(), http.StatusBadRequest)
		return
	}

	res, err := srv.Profile(context.Background(), *param)
	if err != nil {
		body := response{Error: err.Error()}
		if err, ok := err.(ApiError); ok {
			http.Error(w, body.String(), err.HTTPStatus)
			return
		}
		http.Error(w, body.String(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	body := response{Error: "", Response: res}
	w.Write([]byte(body.String()))

}

func (srv *MyApi) CreateHandler(w http.ResponseWriter, r *http.Request) {
	var param *CreateParams = &CreateParams{}

	if r.Method != "POST" {
		err := response{Error: "bad method"}
		http.Error(w, err.String(), http.StatusNotAcceptable)
		return
	}

	if r.Header.Get("X-Auth") != "100500" {
		body := response{Error: "unauthorized"}
		http.Error(w, body.String(), http.StatusForbidden)
		return
	}

	r.ParseForm()
	urlVal := r.Form

	err := param.Unmarshall(urlVal)
	if err != nil {
		res := response{Error: err.Error()}
		http.Error(w, res.String(), http.StatusBadRequest)
		return
	}

	res, err := srv.Create(context.Background(), *param)
	if err != nil {
		body := response{Error: err.Error()}
		if err, ok := err.(ApiError); ok {
			http.Error(w, body.String(), err.HTTPStatus)
			return
		}
		http.Error(w, body.String(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	body := response{Error: "", Response: res}
	w.Write([]byte(body.String()))

}

func (srv *OtherApi) CreateHandler(w http.ResponseWriter, r *http.Request) {
	var param *OtherCreateParams = &OtherCreateParams{}

	if r.Method != "POST" {
		err := response{Error: "bad method"}
		http.Error(w, err.String(), http.StatusNotAcceptable)
		return
	}

	if r.Header.Get("X-Auth") != "100500" {
		body := response{Error: "unauthorized"}
		http.Error(w, body.String(), http.StatusForbidden)
		return
	}

	r.ParseForm()
	urlVal := r.Form

	err := param.Unmarshall(urlVal)
	if err != nil {
		res := response{Error: err.Error()}
		http.Error(w, res.String(), http.StatusBadRequest)
		return
	}

	res, err := srv.Create(context.Background(), *param)
	if err != nil {
		body := response{Error: err.Error()}
		if err, ok := err.(ApiError); ok {
			http.Error(w, body.String(), err.HTTPStatus)
			return
		}
		http.Error(w, body.String(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	body := response{Error: "", Response: res}
	w.Write([]byte(body.String()))

}

func (in *ProfileParams) Unmarshall(q url.Values) error {

	in.Login = q.Get("login")

	if in.Login == "" {
		return fmt.Errorf("login must me not empty")
	}

	return nil
}

func (in *CreateParams) Unmarshall(q url.Values) error {

	in.Login = q.Get("login")

	if in.Login == "" {
		return fmt.Errorf("login must me not empty")
	}

	if len(in.Login) < 10 {
		return fmt.Errorf("login len must be >= 10")
	}

	in.Name = q.Get("full_name")

	in.Status = q.Get("status")

	if in.Status == "" {
		in.Status = "user"
	}

	if !strings.Contains("|user|moderator|admin|", "|"+in.Status+"|") {
		enum := "[" + strings.Replace("user|moderator|admin", "|", ", ", -1) + "]"
		return fmt.Errorf("status must be one of " + enum)
	}

	pAge, err := strconv.Atoi(q.Get("age"))
	if err != nil {
		return fmt.Errorf("age must be int")
	}
	in.Age = pAge

	if in.Age < 0 {
		return fmt.Errorf("age must be >= 0")
	}

	if in.Age > 128 {
		return fmt.Errorf("age must be <= 128")
	}

	return nil
}

func (in *OtherCreateParams) Unmarshall(q url.Values) error {

	in.Username = q.Get("username")

	if in.Username == "" {
		return fmt.Errorf("username must me not empty")
	}

	if len(in.Username) < 3 {
		return fmt.Errorf("username len must be >= 3")
	}

	in.Name = q.Get("account_name")

	in.Class = q.Get("class")

	if in.Class == "" {
		in.Class = "warrior"
	}

	if !strings.Contains("|warrior|sorcerer|rouge|", "|"+in.Class+"|") {
		enum := "[" + strings.Replace("warrior|sorcerer|rouge", "|", ", ", -1) + "]"
		return fmt.Errorf("class must be one of " + enum)
	}

	pLevel, err := strconv.Atoi(q.Get("level"))
	if err != nil {
		return fmt.Errorf("level must be int")
	}
	in.Level = pLevel

	if in.Level < 1 {
		return fmt.Errorf("level must be >= 1")
	}

	if in.Level > 50 {
		return fmt.Errorf("level must be <= 50")
	}

	return nil
}

func (srv *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/profile":
		srv.ProfileHandler(w, r)
	case "/user/create":
		srv.CreateHandler(w, r)
	default:
		err := response{Error: "unknown method"}
		http.Error(w, err.String(), http.StatusNotFound)
	}
}

func (srv *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		srv.CreateHandler(w, r)
	default:
		err := response{Error: "unknown method"}
		http.Error(w, err.String(), http.StatusNotFound)
	}
}
