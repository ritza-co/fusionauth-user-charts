package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/FusionAuth/go-client/pkg/fusionauth"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	var fusionauthUrl = os.Getenv("FUSIONAUTH_URL")
	var apiKey = os.Getenv("API_KEY")
	var applicationId = os.Getenv("APPLICATION_ID")
	baseURL, _ := url.Parse(fusionauthUrl)
	client := fusionauth.NewClient(http.DefaultClient, baseURL, apiKey)

	allUsers := extractUsers(client)
	fmt.Printf("Got all %d users\n", len(allUsers))
	rawJson, _ := json.MarshalIndent(allUsers, "", "\t")
	os.WriteFile("faUsers.json", rawJson, 0644)
	fmt.Println("Wrote FA users to faUsers.json")

	sampleSize := 100
	if len(allUsers) < sampleSize {
		sampleSize = len(allUsers)
	}
	t0 := time.Now()
	getUsersFromFaUsersPerUser(client, allUsers[:sampleSize], applicationId)
	perUserSample := time.Since(t0)
	projected := perUserSample * time.Duration(len(allUsers)) / time.Duration(sampleSize)
	fmt.Printf("Per-user (%d sample): %s -> projected %s for %d users\n", sampleSize, perUserSample.Round(time.Millisecond), projected.Round(time.Millisecond), len(allUsers))

	t0 = time.Now()
	extractedUsers := getUsersFromFaUsers(client, allUsers, applicationId)
	bulkDuration := time.Since(t0)
	fmt.Printf("Bulk: %s for %d users\n", bulkDuration.Round(time.Millisecond), len(extractedUsers))

	finalJson, _ := json.MarshalIndent(extractedUsers, "", "\t")
	os.WriteFile("users.json", finalJson, 0644)
	fmt.Printf("Wrote %d extracted users to users.json\n", len(extractedUsers))
}

func extractUsers(client *fusionauth.FusionAuthClient) []fusionauth.User {
	allUsers := []fusionauth.User{}
	pageSize := 1000
	nextResults := ""
	for {
		searchReq := fusionauth.SearchRequest{
			Search: fusionauth.UserSearchCriteria{},
		}
		searchReq.Search.NumberOfResults = pageSize
		if nextResults != "" {
			searchReq.Search.NextResults = nextResults
		} else {
			searchReq.Search.QueryString = "*"
		}
		resp, errors, err := client.SearchUsersByQuery(searchReq)
		if err != nil {
			fmt.Println(err.Error())
			return allUsers
		}
		if errors != nil {
			fmt.Printf("httpError: %s\n", *errors)
			return allUsers
		}
		allUsers = append(allUsers, resp.Users...)
		fmt.Printf("Extracted %d users (%d total)\n", len(resp.Users), len(allUsers))
		if len(resp.Users) < pageSize {
			break
		}
		if resp.NextResults == "" {
			fmt.Println("Warning: no nextResults token returned; cannot paginate past 10,000 users")
			break
		}
		nextResults = resp.NextResults
	}
	return allUsers
}

func getUsersFromFaUsersPerUser(client *fusionauth.FusionAuthClient, faUsers []fusionauth.User, applicationId string) []UserOutput {
	unverifiedReasons := []string{"Completed", "Implicit", "Pending"}
	var users []UserOutput
	for _, faUser := range faUsers {
		var identity *fusionauth.UserIdentity
		for i := range faUser.Identities {
			if faUser.Identities[i].Primary {
				identity = &faUser.Identities[i]
				break
			}
		}
		var registration *fusionauth.UserRegistration
		for i := range faUser.Registrations {
			if faUser.Registrations[i].ApplicationId == applicationId {
				registration = &faUser.Registrations[i]
				break
			}
		}
		if registration == nil {
			continue
		}
		isVerified := faUser.Verified
		if identity != nil {
			isVerified = identity.Verified || !contains(unverifiedReasons, string(identity.VerifiedReason))
		}
		user := UserOutput{
			Id:             faUser.Id,
			Email:          faUser.Email,
			IsVerified:     isVerified,
			RegisteredDate: registration.InsertInstant,
			LoginDates:     []int64{},
		}
		loginSearchReq := fusionauth.LoginRecordSearchRequest{
			Search: fusionauth.LoginRecordSearchCriteria{
				UserId:        faUser.Id,
				ApplicationId: applicationId,
				BaseSearchCriteria: fusionauth.BaseSearchCriteria{
					NumberOfResults: 10000,
				},
			},
		}
		loginResp, _, err := client.SearchLoginRecords(loginSearchReq)
		if err == nil && loginResp != nil {
			for _, l := range loginResp.Logins {
				user.LoginDates = append(user.LoginDates, l.Instant)
			}
		}
		sort.Slice(user.LoginDates, func(i, j int) bool { return user.LoginDates[i] < user.LoginDates[j] })
		if len(user.LoginDates) > 0 && user.LoginDates[0] == user.RegisteredDate {
			user.LoginDates = user.LoginDates[1:]
		}
		users = append(users, user)
	}
	return users
}

func getUsersFromFaUsers(client *fusionauth.FusionAuthClient, faUsers []fusionauth.User, applicationId string) []UserOutput {
	unverifiedReasons := []string{"Completed", "Implicit", "Pending"}

	byId := make(map[string]*UserOutput)
	for _, faUser := range faUsers {
		var identity *fusionauth.UserIdentity
		for i := range faUser.Identities {
			if faUser.Identities[i].Primary {
				identity = &faUser.Identities[i]
				break
			}
		}
		var registration *fusionauth.UserRegistration
		for i := range faUser.Registrations {
			if faUser.Registrations[i].ApplicationId == applicationId {
				registration = &faUser.Registrations[i]
				break
			}
		}
		if registration == nil {
			continue
		}
		isVerified := faUser.Verified
		if identity != nil {
			isVerified = identity.Verified || !contains(unverifiedReasons, string(identity.VerifiedReason))
		}
		u := &UserOutput{
			Id:             faUser.Id,
			Email:          faUser.Email,
			IsVerified:     isVerified,
			RegisteredDate: registration.InsertInstant,
			LoginDates:     []int64{},
		}
		byId[faUser.Id] = u
	}

	const pageSize = 10000
	fmt.Printf("Fetching login records for %d users (bulk by application)...\n", len(byId))
	for startRow := 0; ; startRow += pageSize {
		req := fusionauth.LoginRecordSearchRequest{
			Search: fusionauth.LoginRecordSearchCriteria{
				ApplicationId: applicationId,
				BaseSearchCriteria: fusionauth.BaseSearchCriteria{
					NumberOfResults: pageSize,
					StartRow:        startRow,
				},
			},
		}
		resp, _, err := client.SearchLoginRecords(req)
		if err != nil || resp == nil {
			break
		}
		fmt.Printf("  page startRow=%d: got %d records\n", startRow, len(resp.Logins))
		for _, l := range resp.Logins {
			if u, ok := byId[l.UserId]; ok {
				u.LoginDates = append(u.LoginDates, l.Instant)
			}
		}
		if len(resp.Logins) < pageSize {
			break
		}
	}

	users := make([]UserOutput, 0, len(byId))
	for _, u := range byId {
		sort.Slice(u.LoginDates, func(i, j int) bool { return u.LoginDates[i] < u.LoginDates[j] })
		if len(u.LoginDates) > 0 && u.LoginDates[0] == u.RegisteredDate {
			u.LoginDates = u.LoginDates[1:]
		}
		users = append(users, *u)
	}
	return users
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

type UserOutput struct {
	Id             string  `json:"id"`
	Email          string  `json:"email"`
	IsVerified     bool    `json:"isVerified"`
	RegisteredDate int64   `json:"registeredDate"`
	LoginDates     []int64 `json:"loginDates"`
}
