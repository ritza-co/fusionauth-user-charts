// docker run --init  -it  --rm --platform linux/amd64 --name "app" --network faNetwork -v .:/app -v ./gocache:/go/pkg -v ./buildcache:/root/.cache/go-build -w /app golang:1.25-bookworm sh -c "go fmt extractUsers.go && go run extractUsers.go"

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"

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

	allUsers := []fusionauth.User{}
	start := 0
	pageSize := 1000
	for {
		searchReq := fusionauth.SearchRequest{
			Search: fusionauth.UserSearchCriteria{},
		}
		searchReq.Search.QueryString = "*"
		searchReq.Search.StartRow = start
		searchReq.Search.NumberOfResults = pageSize
		resp, errors, err := client.SearchUsersByQuery(searchReq)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		if errors != nil {
			fmt.Printf("httpError: %s\n", *errors)
			return
		}

		allUsers = append(allUsers, resp.Users...)
		fmt.Printf("Extracted %d users (%d total)\n", len(resp.Users), len(allUsers))
		if len(resp.Users) < pageSize {
			break
		}
		start += pageSize
	}

	fmt.Printf("Got all %d users\n", len(allUsers))
	rawJson, _ := json.MarshalIndent(allUsers, "", "\t")
	os.WriteFile("faUsers.json", rawJson, 0644)
	fmt.Println("Wrote FA users to faUsers.json")

	extractedUsers := getUsersFromFaUsers(client, allUsers, applicationId)
	finalJson, _ := json.MarshalIndent(extractedUsers, "", "\t")
	os.WriteFile("users.json", finalJson, 0644)
	fmt.Printf("Wrote %d extracted users to users.json\n", len(extractedUsers))
}

func getUsersFromFaUsers(client *fusionauth.FusionAuthClient, faUsers []fusionauth.User, applicationId string) []UserOutput {
	var users []UserOutput
	unverifiedReasons := []string{"Completed", "Implicit", "Pending"}
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
		if identity == nil || registration == nil {
			continue
		}
		user := UserOutput{
			Id:             faUser.Id,
			Email:          faUser.Email,
			IsVerified:     identity.Verified || !contains(unverifiedReasons, string(identity.VerifiedReason)),
			RegisteredDate: registration.InsertInstant,
			LoginDates:     []int64{},
		}
		loginSearchReq := fusionauth.LoginRecordSearchRequest{
			Search: fusionauth.LoginRecordSearchCriteria{
				UserId: faUser.Id,
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
