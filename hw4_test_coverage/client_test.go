package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type Row struct {
	Text          string `xml:",chardata" json:"-"`
	ID            int    `xml:"id" json:"id"`
	GUID          string `xml:"-" json:"guid"`
	IsActive      string `xml:"-"  json:"is_active"`
	Balance       string `xml:"-" json:"balance"`
	Picture       string `xml:"-" json:"picture"`
	Age           int    `xml:"age" json:"age"`
	EyeColor      string `xml:"-" json:"eye_color"`
	FirstName     string `xml:"first_name" json:"first_name"`
	LastName      string `xml:"last_name" json:"last_name"`
	Gender        string `xml:"gender" json:"gender"`
	Company       string `xml:"-" json:"company"`
	Email         string `xml:"-" json:"email"`
	Phone         string `xml:"-" json:"phone"`
	Address       string `xml:"-" json:"address"`
	About         string `xml:"about" json:"about"`
	Registered    string `xml:"-" json:"registered"`
	FavoriteFruit string `xml:"-" json:"favorite_fruit"`
}

type Root struct {
	XMLName xml.Name `xml:"root"`
	Text    string   `xml:",chardata"`
	Rows    []Row    `xml:"row"`
}

func FilterByQuery(query string, rows []Row) []Row {
	res := make([]Row, 0, len(rows))
	if query == "" {
		return rows
	}

	for _, row := range rows {
		if strings.Contains(row.FirstName+" "+row.LastName, query) || strings.Contains(row.About, query) {
			res = append(res, row)
		}
	}

	return res
}
func SearchService(w http.ResponseWriter, r *http.Request) {
	session := r.Header.Get("AccessToken")
	if session == "" {
		http.Error(w, fmt.Sprintf("Bad AccessToken"), http.StatusUnauthorized)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	query := r.URL.Query().Get("query")

	if query == "timeout" {
		time.Sleep(2 * time.Second)
	}
	if query == "wrong_json" {
		writeBadError(w)
		return
	}
	if query == "internal" {
		http.Error(w, "TestInternalServerError", http.StatusInternalServerError)
		return
	}
	if query == "bad_user" {
		json.NewEncoder(w).Encode(`dermeco`)
		return
	}

	orderField := r.URL.Query().Get("order_field")
	orderBy, err := strconv.Atoi(r.URL.Query().Get("order_by"))
	if err != nil {
		writeError(w, "order_by should be integer")
		return
	}
	if orderBy != 1 && orderBy != 0 && orderBy != -1 {
		writeError(w, "order_by should be 1,0 or -1")
		return
	}
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil {
		writeError(w, "offset should be integer")
		return
	}
	if offset < 0 {
		writeError(w, "offset should >  0")
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, "limit should be integer")
		return
	}

	data, err := ioutil.ReadFile("dataset.xml")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	root := &Root{}
	err = xml.Unmarshal(data, root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	res := FilterByQuery(query, root.Rows)

	err = sorting(res, query, orderField, orderBy)
	if err != nil {
		writeError(w, err.Error())
		return
	}

	// for i, row := range res {
	// 	fmt.Println(i, row.ID, row.FirstName, row.LastName, row.Age)
	// }
	if offset >= len(res) {
		w.Write(nil)
	}
	res = res[offset:]
	if limit < len(res) {
		res = res[:limit]
	}
	u := make([]User, 0, len(res))
	for i := range res {
		u = append(u, *toUser(&res[i]))
	}

	err = json.NewEncoder(w).Encode(u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
func writeBadError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	err := json.NewEncoder(w).Encode(`{BadJson}`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, errText string) {
	w.WriteHeader(http.StatusBadRequest)
	err := json.NewEncoder(w).Encode(&SearchErrorResponse{Error: errText})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func toUser(r *Row) *User {
	return &User{
		Id:     r.ID,
		Name:   r.FirstName + " " + r.LastName,
		Age:    r.Age,
		About:  r.About,
		Gender: r.Gender,
	}
}

func sorting(rows []Row, query, orderField string, orderBy int) error {
	switch orderField {
	case "Name", "":
		switch orderBy {
		case -1:
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].FirstName+" "+rows[i].LastName < rows[j].FirstName+" "+rows[j].LastName
			})
		case 1:
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].FirstName+" "+rows[i].LastName > rows[j].FirstName+" "+rows[j].LastName
			})
		default:
			return nil
		}

	case "Id":
		switch orderBy {
		case -1:
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].ID < rows[j].ID
			})
		case 1:
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].ID > rows[j].ID
			})
		default:
			return nil
		}
	case "Age":
		switch orderBy {
		case -1:
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].Age < rows[j].Age
			})
		case 1:
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].Age > rows[j].Age
			})
		default:
			return nil
		}
	default:
		return fmt.Errorf("ErrorBadOrderField")
	}
	return nil
}

type TestCase struct {
	Request *SearchRequest
	Result  []int
	IsError bool
}

func TestFindUsers(t *testing.T) {
	cases := []*TestCase{
		{
			Request: &SearchRequest{
				Limit:      -1,
				Offset:     5,
				Query:      "voluptate",
				OrderField: "Id",
				OrderBy:    -1,
			},
			Result:  nil,
			IsError: true,
		},
		{
			Request: &SearchRequest{
				Limit:      3,
				Offset:     0,
				Query:      "voluptate",
				OrderField: "Id",
				OrderBy:    -1,
			},
			Result:  []int{0, 1, 2},
			IsError: false,
		},
		{
			Request: &SearchRequest{
				Limit:      26,
				Offset:     10,
				Query:      "voluptate",
				OrderField: "Id",
				OrderBy:    -1,
			},
			Result:  []int{11, 12, 13, 15, 17, 19, 22, 25, 26, 27, 28, 29, 30, 31},
			IsError: false,
		},
		{
			Request: &SearchRequest{
				Limit:      26,
				Offset:     -1,
				Query:      "voluptate",
				OrderField: "Id",
				OrderBy:    -1,
			},
			Result:  nil,
			IsError: true,
		},
	}
	s := httptest.NewServer(http.HandlerFunc(SearchService))
	for i, item := range cases {
		srv := &SearchClient{
			AccessToken: "123",
			URL:         s.URL,
		}
		result, err := srv.FindUsers(*item.Request)
		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error: %#v", i, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil", i)
		}
		if err == nil && !item.IsError {
			if len(item.Result) != len(result.Users) {
				// for _, k := range result.Users {
				// 	fmt.Printf("%d ", k.Id)
				// }
				// fmt.Println()
				t.Errorf("[%d] wrong result, expected length %#v, got %#v", i, len(item.Result), len(result.Users))
			}
			for j := range item.Result {
				if item.Result[j] != result.Users[j].Id {
					t.Errorf("[%d] wrong result, expected %#v, got %#v", i, item.Result[j], result.Users[j].Id)
				}
			}
		}
	}
}

type TestAuth struct {
	Client  *SearchClient
	Request *SearchRequest
	Result  []int
	IsError bool
}

func TestFindUsersAuth(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(SearchService))
	cases := []*TestAuth{
		{
			Client: &SearchClient{
				AccessToken: "",
				URL:         s.URL,
			},
			Request: &SearchRequest{
				Limit:      0,
				Offset:     5,
				Query:      "voluptate",
				OrderField: "Id",
				OrderBy:    -1,
			},
			Result:  nil,
			IsError: true,
		},
	}
	for i, item := range cases {
		_, err := item.Client.FindUsers(*item.Request)
		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error: %#v", i, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil", i)
		}
	}
}

func TestFindUsersGeneral(t *testing.T) {
	cases := []*TestCase{
		{
			Request: &SearchRequest{
				Limit:      5,
				Offset:     5,
				Query:      "timeout",
				OrderField: "Id",
				OrderBy:    -1,
			},
			Result:  nil,
			IsError: true,
		},
		{
			Request: &SearchRequest{
				Limit:      5,
				Offset:     5,
				Query:      "internal",
				OrderField: "Id",
				OrderBy:    -1,
			},
			Result:  nil,
			IsError: true,
		},
	}
	s := httptest.NewServer(http.HandlerFunc(SearchService))
	for i, item := range cases {
		srv := &SearchClient{
			AccessToken: "123",
			URL:         s.URL,
		}
		result, err := srv.FindUsers(*item.Request)
		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error: %#v", i, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil", i)
		}
		if err == nil && !item.IsError {
			if len(item.Result) != len(result.Users) {
				t.Errorf("[%d] wrong result, expected length %#v, got %#v", i, len(item.Result), len(result.Users))
			}
			for j := range item.Result {
				if item.Result[j] != result.Users[j].Id {
					t.Errorf("[%d] wrong result, expected %#v, got %#v", i, item.Result[j], result.Users[j].Id)
				}
			}
		}
	}
}

func TestFindUsersBadRequest(t *testing.T) {
	cases := []*TestCase{
		{
			Request: &SearchRequest{
				Limit:      5,
				Offset:     5,
				Query:      "voluptate",
				OrderField: "Favno",
				OrderBy:    -1,
			},
			Result:  nil,
			IsError: true,
		},
		{
			Request: &SearchRequest{
				Limit:      5,
				Offset:     0,
				Query:      "wrong_json",
				OrderField: "Name",
				OrderBy:    -1,
			},
			Result:  nil,
			IsError: true,
		},
		{
			Request: &SearchRequest{
				Limit:      5,
				Offset:     0,
				Query:      "voluptate",
				OrderField: "Name",
				OrderBy:    2,
			},
			Result:  nil,
			IsError: true,
		},
		{
			Request: &SearchRequest{
				Limit:      5,
				Offset:     0,
				Query:      "bad_user",
				OrderField: "Name",
				OrderBy:    2,
			},
			Result:  nil,
			IsError: true,
		},
	}
	s := httptest.NewServer(http.HandlerFunc(SearchService))
	for i, item := range cases {
		srv := &SearchClient{
			AccessToken: "123",
			URL:         s.URL,
		}
		_, err := srv.FindUsers(*item.Request)
		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error: %#v", i, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil", i)
		}
	}
}

func TestFindUsersUnknownError(t *testing.T) {
	cases := []*TestCase{
		{
			Request: &SearchRequest{
				Limit:      3,
				Offset:     0,
				Query:      "voluptate",
				OrderField: "Id",
				OrderBy:    -1,
			},
			Result:  []int{0, 1, 2},
			IsError: true,
		},
	}
	s := httptest.NewServer(http.HandlerFunc(SearchService))
	s.Close()
	for i, item := range cases {
		srv := &SearchClient{
			AccessToken: "123",
			URL:         s.URL,
		}
		result, err := srv.FindUsers(*item.Request)
		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error: %#v", i, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil", i)
		}
		if err == nil && !item.IsError {
			if len(item.Result) != len(result.Users) {
				t.Errorf("[%d] wrong result, expected length %#v, got %#v", i, len(item.Result), len(result.Users))
			}
			for j := range item.Result {
				if item.Result[j] != result.Users[j].Id {
					t.Errorf("[%d] wrong result, expected %#v, got %#v", i, item.Result[j], result.Users[j].Id)
				}
			}
		}
	}
}
