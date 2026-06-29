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

	err = json.NewEncoder(w).Encode(filteredUsers)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func TestFindUser(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		request       SearchRequest
		expectedError bool
	}{
		{
			"Normal request",
			validAccessToken,
			SearchRequest{
				Limit:      2,
				Offset:     0,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByAsc},
			false,
		},
		{
			"Negative limit",
			validAccessToken,
			SearchRequest{
				Limit:      -1,
				Offset:     0,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByAsc},
			true,
		},
		{
			"Limit more then 25",
			validAccessToken,
			SearchRequest{
				Limit:      26,
				Offset:     0,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByAsc},
			false,
		},
		{
			"Negative offset",
			validAccessToken,
			SearchRequest{
				Limit:      2,
				Offset:     -1,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByAsc},
			true,
		},
		{
			"Bad access token",
			"non-valid-access-token",
			SearchRequest{
				Limit:      2,
				Offset:     0,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByAsc},
			true,
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			searchClient := &SearchClient{
				AccessToken: test.token,
				URL:         ts.URL,
			}

			result, err := searchClient.FindUsers(test.request)
			_ = result

			if !test.expectedError {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Unexpected success")
				}
			}
		})
	}
}

func TestFindUserLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	searchClient := &SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}
	result, err := searchClient.FindUsers(SearchRequest{
		Limit:      -1,
		Offset:     0,
		Query:      "",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})
	if err.Error() != "limit must be > 0" {
		t.Fatalf("expected limit must be > 0, got: %v", err)
	}

	result, err = searchClient.FindUsers(SearchRequest{
		Limit:      26,
		Offset:     0,
		Query:      "",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(result.Users) != 25 {
		t.Fatalf("expected 25 users, got %d", len(result.Users))
	}
}

func TestFindUserOffset(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	searchClient := &SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}
	_, err := searchClient.FindUsers(SearchRequest{
		Limit:      2,
		Offset:     -1,
		Query:      "",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})
	if err.Error() != "offset must be > 0" {
		t.Fatalf("expected offset must be > 0, got: %v", err)
	}
}

func TestFindUserQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	searchClient := &SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}
	result, err := searchClient.FindUsers(SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "Boyd Wolf",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Users[0].Name != "Boyd Wolf" {
		t.Fatalf("expected Boyd Wolf, got %v", result.Users[0].Name)
	}

	about := "Dolore magna magna commodo irure. Proident culpa nisi veniam excepteur sunt qui et laborum tempor. Qui proident Lorem commodo dolore ipsum.\n"
	result, err = searchClient.FindUsers(SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      about,
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Users[0].About != about {
		t.Fatalf("expected %v, got %v", about, result.Users[0].About)
	}
}

func TestFindUserOrderField(t *testing.T) {
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
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Users[0].Id != 0 {
		t.Fatalf("expected first user id 0, got %v", result.Users[0].Id)
	}
	if result.Users[1].Id != 1 {
		t.Fatalf("expected second user id 1, got %v", result.Users[1].Id)
	}

	result, err = searchClient.FindUsers(SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "",
		OrderField: "Age",
		OrderBy:    OrderByAsc,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Users[0].Age != 21 {
		t.Fatalf("expected first user age 1, got %v", result.Users[0].Age)
	}
	if result.Users[1].Age != 21 {
		t.Fatalf("expected second user age 1, got %v", result.Users[1].Age)
	}
}

func TestFindUserBadAccessToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	searchClient := &SearchClient{
		AccessToken: "non-valid-access-token",
		URL:         ts.URL,
	}
	_, err := searchClient.FindUsers(SearchRequest{})

	if err.Error() != "Bad AccessToken" {
		t.Fatalf("expected Bad AccessToken, got %v", err)
	}
}

func TestFindUserUnmarshalJson(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	searchClient := &SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}

	result, err := searchClient.FindUsers(SearchRequest{})
	_ = result
	_ = err
}
