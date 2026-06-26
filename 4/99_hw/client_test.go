package main

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type XMLUser struct {
	ID        int    `xml:"id"`
	Age       int    `xml:"age"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Gender    string `xml:"gender"`
	About     string `xml:"about"`
}

type XMLUsers struct {
	List []XMLUser `xml:"row"`
}

const validAccessToken = "valid-access-token"

func SearchServer(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("AccessToken")
	if token != validAccessToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	query := r.FormValue("query")
	orderField := r.FormValue("order_field")
	limit, err := strconv.Atoi(r.FormValue("limit"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	offset, err := strconv.Atoi(r.FormValue("offset"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	orderBy, err := strconv.Atoi(r.FormValue("order_by"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	data, err := os.ReadFile("dataset.xml")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var xmlUsers XMLUsers
	err = xml.Unmarshal(data, &xmlUsers)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	users := make([]User, len(xmlUsers.List))
	for i, xmlUser := range xmlUsers.List {
		users[i] = User{
			Id:     xmlUser.ID,
			Name:   xmlUser.FirstName + " " + xmlUser.LastName,
			Age:    xmlUser.Age,
			About:  xmlUser.About,
			Gender: xmlUser.Gender,
		}
	}

	filteredUsers := make([]User, 0, len(users))

	for _, user := range users {
		if query == "" || strings.Contains(user.Name, query) || strings.Contains(user.About, query) {
			filteredUsers = append(filteredUsers, user)
		}
	}

	switch orderBy {
	case OrderByAsc:
		switch orderField {
		case "Id":
			sort.Slice(filteredUsers, func(i, j int) bool {
				return filteredUsers[i].Id < filteredUsers[j].Id
			})
		case "Age":
			sort.Slice(filteredUsers, func(i, j int) bool {
				return filteredUsers[i].Age < filteredUsers[j].Age
			})
		case "", "Name":
			sort.Slice(filteredUsers, func(i, j int) bool {
				return filteredUsers[i].Name < filteredUsers[j].Name
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	case OrderByDesc:
		switch orderField {
		case "Id":
			sort.Slice(filteredUsers, func(i, j int) bool {
				return filteredUsers[i].Id > filteredUsers[j].Id
			})
		case "Age":
			sort.Slice(filteredUsers, func(i, j int) bool {
				return filteredUsers[i].Age > filteredUsers[j].Age
			})
		case "", "Name":
			sort.Slice(filteredUsers, func(i, j int) bool {
				return filteredUsers[i].Name > filteredUsers[j].Name
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	case OrderByAsIs:
		switch orderField {
		case "Id", "Age", "Name", "":

		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	if offset > len(filteredUsers) {
		filteredUsers = []User{}
	} else {
		filteredUsers = filteredUsers[offset:]
	}
	if limit < len(filteredUsers) {
		filteredUsers = filteredUsers[:limit]
	}

	json.NewEncoder(w).Encode(filteredUsers)
}

func TestFindUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	searchClient := &SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}
	result, err := searchClient.FindUsers(SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(result.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(result.Users))
	}

	if result.Users[0].Id != 0 {
		t.Fatalf("expected first user id 0, got %d", result.Users[0].Id)
	}

	if result.Users[1].Id != 1 {
		t.Fatalf("expected second user id 1, got %d", result.Users[1].Id)
	}

	if !result.NextPage {
		t.Fatalf("expected NextPage = true")
	}
}
