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

	users, err := getResFromDb()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	filters := SearchRequest{
		Query:      query,
		OrderField: orderField,
		Limit:      limit,
		Offset:     offset,
		OrderBy:    orderBy,
	}
	res, err := filterData(users, filters)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errResp := SearchErrorResponse{
			Error: fmt.Errorf("SearchServer fatal error").Error(),
		}
		jsonErr, _ := json.Marshal(errResp)
		_, _ = fmt.Fprintln(w, string(jsonErr))
		c.JSON(http.StatusOK, response)
		return
	}

	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func getResFromDb() ([]User, error) {
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

	switch filters.OrderBy {
	case OrderByAsc:
		switch filters.OrderField {
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
			return nil, fmt.Errorf("invalid order_field: %s", filters.OrderField)
		}
	case OrderByDesc:
		switch filters.OrderField {
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
			return nil, fmt.Errorf("invalid order_field: %s", filters.OrderField)
		}
	case OrderByAsIs:
		switch filters.OrderField {
		case "Id", "Age", "Name", "":
		default:
			return nil, fmt.Errorf("invalid order_field: %s", filters.OrderField)
		}
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

func TestFindUser(t *testing.T) {
	type testIN struct {
		token   string
		request SearchRequest
	}

	type testOut struct {
		expectedInError string
		errResp         SearchErrorResponse
	}

	tests := []struct {
		name string
		in   testIN
		out  testOut
	}{
		{
			"Normal request",
			testIN{validAccessToken,
				SearchRequest{
					Limit:      2,
					Offset:     0,
					Query:      "",
					OrderField: "Id",
					OrderBy:    OrderByAsc}},
			testOut{
				expectedInError: "",
				errResp:         SearchErrorResponse{},
			},
		},
		{
			"Negative limit",
			testIN{validAccessToken,
				SearchRequest{
					Limit:      -1,
					Offset:     0,
					Query:      "",
					OrderField: "Id",
					OrderBy:    OrderByAsc}},
			testOut{
				expectedInError: "limit must be > 0",
			},
		},
		//{
		//	"Limit more then 25",
		//	validAccessToken,
		//	SearchRequest{
		//		Limit:      26,
		//		Offset:     0,
		//		Query:      "",
		//		OrderField: "Id",
		//		OrderBy:    OrderByAsc},
		//	false,
		//},
		//{
		//	"Negative offset",
		//	validAccessToken,
		//	SearchRequest{
		//		Limit:      2,
		//		Offset:     -1,
		//		Query:      "",
		//		OrderField: "Id",
		//		OrderBy:    OrderByAsc},
		//	true,
		//},
		//{
		//	"Bad access token",
		//	"non-valid-access-token",
		//	SearchRequest{
		//		Limit:      2,
		//		Offset:     0,
		//		Query:      "",
		//		OrderField: "Id",
		//		OrderBy:    OrderByAsc},
		//	true,
		//},
	}

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	users, _ := getResFromDb()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testUsers := slices.Clone(users)
			searchClient := &SearchClient{
				AccessToken: test.in.token,
				URL:         ts.URL,
			}

			result, err := searchClient.FindUsers(test.in.request)

			if test.out.expectedInError != "" {
				assert.ErrorContains(t, err, test.out.expectedInError)
				return
			}

			expected := makeResponse(testUsers, test.in.request)
			assert.Equal(t, expected, result)
		})
	}
}

func makeResponse(users []User, filters SearchRequest) *SearchResponse {
	resp := &SearchResponse{}
	filtered, _ := filterData(users, filters)
	if len(filtered) == filters.Limit {
		resp.NextPage = true
	}
	resp.Users = filtered[0:len(filtered)]

	return resp
}
