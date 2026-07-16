package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SearchErrorResponse{
			Error: "invalid limit",
		})
		return
	}
	offset, err := strconv.Atoi(r.FormValue("offset"))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SearchErrorResponse{
			Error: "invalid offset",
		})
		return
	}
	orderBy, err := strconv.Atoi(r.FormValue("order_by"))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SearchErrorResponse{
			Error: "invalid order_by",
		})
		return
	}
	users, err := getDataFromDb()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	filters := SearchRequest{
		Limit:      limit,
		Offset:     offset,
		Query:      query,
		OrderField: orderField,
		OrderBy:    orderBy,
	}
	filteredData, err := filterData(users, filters)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SearchErrorResponse{
			Error: err.Error(),
		})
		return
	}

	err = json.NewEncoder(w).Encode(filteredData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func getDataFromDb() ([]User, error) {
	data, err := os.ReadFile("dataset.xml")
	if err != nil {
		return nil, fmt.Errorf("failed to read dataset.xml: %w", err)
	}

	var xmlUsers XMLUsers
	if err := xml.Unmarshal(data, &xmlUsers); err != nil {
		return nil, fmt.Errorf("error unmarshalling XML: %v", err)
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
	return users, nil
}

func filterData(users []User, filters SearchRequest) ([]User, error) {
	filteredUsers := make([]User, 0, len(users))

	for _, user := range users {
		if filters.Query == "" || strings.Contains(user.Name, filters.Query) || strings.Contains(user.About, filters.Query) {
			filteredUsers = append(filteredUsers, user)
		}
	}

	if filters.OrderBy != OrderByAsIs {
		sortFunc, err := makeSortFunc(filteredUsers, filters.OrderField, filters.OrderBy)
		if err != nil {
			return nil, err
		}
		sort.Slice(filteredUsers, sortFunc)
	}

	if filters.Offset > len(filteredUsers) {
		filteredUsers = []User{}
	} else {
		filteredUsers = filteredUsers[filters.Offset:]
	}
	if filters.Limit < len(filteredUsers) {
		filteredUsers = filteredUsers[:filters.Limit]
	}
	return filteredUsers, nil
}

func makeSortFunc(users []User, orderField string, orderBy int) (func(i, j int) bool, error) {
	switch orderField {
	case "Id":
		return func(i, j int) bool {
			if orderBy == OrderByAsc {
				return users[i].Id < users[j].Id
			}
			return users[i].Id > users[j].Id
		}, nil
	case "Age":
		return func(i, j int) bool {
			if orderBy == OrderByAsc {
				return users[i].Age < users[j].Age
			}
			return users[i].Age > users[j].Age
		}, nil
	case "", "Name":
		return func(i, j int) bool {
			if orderBy == OrderByAsc {
				return users[i].Name < users[j].Name
			}
			return users[i].Name > users[j].Name
		}, nil
	default:
		return nil, fmt.Errorf("ErrorBadOrderField")
	}
}

func makeResponse(users []User, filters SearchRequest) *SearchResponse {
	resp := &SearchResponse{}
	if filters.Limit > 25 {
		filters.Limit = 25
	}

	filtered, _ := filterData(users, filters)
	if len(filtered) == filters.Limit {
		resp.NextPage = true
	}
	resp.Users = filtered[0:]

	return resp
}

func TestFindUser(t *testing.T) {
	type testIn struct {
		token   string
		request SearchRequest
	}

	type testOut struct {
		expectedError string
		errResp       SearchErrorResponse
	}

	tests := []struct {
		name string
		in   testIn
		out  testOut
	}{
		{
			"Normal request",
			testIn{
				validAccessToken,
				SearchRequest{
					Limit:      5,
					Offset:     1,
					Query:      "Boyd Wolf",
					OrderField: "Id",
					OrderBy:    OrderByDesc},
			},
			testOut{
				expectedError: "",
				errResp:       SearchErrorResponse{},
			},
		},
		{
			"Negative limit",
			testIn{
				validAccessToken,
				SearchRequest{
					Limit:      -1,
					Offset:     0,
					Query:      "",
					OrderField: "Id",
					OrderBy:    OrderByAsc},
			},
			testOut{
				expectedError: "limit must be > 0",
			},
		},
		{
			"Limit more than 25",
			testIn{
				validAccessToken,
				SearchRequest{
					Limit:      26,
					Offset:     0,
					Query:      "",
					OrderField: "Id",
					OrderBy:    OrderByAsc,
				},
			},
			testOut{
				expectedError: "",
				errResp:       SearchErrorResponse{},
			},
		},
		{
			"Negative offset",
			testIn{
				validAccessToken,
				SearchRequest{
					Limit:      2,
					Offset:     -1,
					Query:      "",
					OrderField: "Id",
					OrderBy:    OrderByAsc},
			},
			testOut{
				expectedError: "offset must be > 0",
			},
		},
		{
			"Bad order field",
			testIn{
				validAccessToken,
				SearchRequest{
					Limit:      2,
					Offset:     0,
					Query:      "",
					OrderField: "Bad",
					OrderBy:    OrderByAsc},
			},
			testOut{
				expectedError: "OrderFeld Bad invalid",
			},
		},
		{
			"Bad access token",
			testIn{
				"non-valid-token",
				SearchRequest{},
			},
			testOut{
				expectedError: "Bad AccessToken",
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()
	users, _ := getDataFromDb()
	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {
			testUsers := slices.Clone(users)
			searchClient := &SearchClient{
				AccessToken: test.in.token,
				URL:         ts.URL,
			}

			result, err := searchClient.FindUsers(test.in.request)

			if test.out.expectedError != "" {
				assert.ErrorContains(t, err, test.out.expectedError)
				return
			}

			expected := makeResponse(testUsers, test.in.request)
			assert.Equal(t, expected, result)
		})
	}
}

func TestFindUsersBadJson(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintln(w, "not json")
	}))
	defer ts.Close()
	searchClient := &SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}
	_, err := searchClient.FindUsers(SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})
	assert.ErrorContains(t, err, "cant unpack error json")
}

func TestFindUsersTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		_, _ = fmt.Fprintln(w, "timeout")
	}))
	defer ts.Close()
	searchClient := &SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}
	_, err := searchClient.FindUsers(SearchRequest{})
	assert.ErrorContains(t, err, "timeout for")
}

func TestFindUsersUnknownError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintln(w, `{"error": "bad request"}`)
	}))
	searchClient := &SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}
	ts.Close()
	_, err := searchClient.FindUsers(SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})
	assert.ErrorContains(t, err, "unknown error")
}

func TestFindUsersInternalServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintln(w, "error")
	}))
	defer ts.Close()
	searchClient := &SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}
	_, err := searchClient.FindUsers(SearchRequest{})
	assert.ErrorContains(t, err, "SearchServer fatal error")
}

func TestFindUsersBadRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintln(w, `{"error": "bad request"}`)
	}))
	defer ts.Close()
	searchClient := &SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}
	_, err := searchClient.FindUsers(SearchRequest{})
	assert.ErrorContains(t, err, "unknown bad request error: bad request")
}

func TestFindUsersUnpackJsonError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, `{"error": "bad request"}`)
	}))
	defer ts.Close()
	searchClient := &SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}
	_, err := searchClient.FindUsers(SearchRequest{})
	assert.ErrorContains(t, err, "cant unpack result json:")
}
