// docker run --init  -it  --rm --platform linux/amd64 --name "app" --network faNetwork -v .:/app -v ./gocache:/go/pkg -v ./buildcache:/root/.cache/go-build -w /app golang:1.25-bookworm sh -c "go fmt 3extract.go && go run 3extract.go"

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
)

const applicationId = "e9fdb985-9173-4e01-9d73-ac2d60d1dc8e"
const apiKey = "33052c8a-c283-4e96-9d2a-eb1215c69f8f-not-for-prod"
const faUrl = "http://fa:9011"

func main() {
	var faUserResp FaUserResponse
	getFaData("/api/user/search?queryString=*&numberOfResults=999999&startRow=0", &faUserResp)
	fmt.Printf("Got all %d users\n", len(faUserResp.Users))
	rawJson, _ := json.MarshalIndent(faUserResp.Users, "", "\t")
	os.WriteFile("faUsers.json", rawJson, 0644)
	fmt.Println("Wrote FA users to faUsers.json")

	extractedUsers := getUsersFromFaUsers(faUserResp.Users)
	finalJson, _ := json.MarshalIndent(extractedUsers, "", "\t")
	os.WriteFile("users.json", finalJson, 0644)
	fmt.Printf("Wrote %d extracted users to users.json\n", len(extractedUsers))
}

func getUsersFromFaUsers(faUsers []FaUser) []UserOutput {
	var users []UserOutput
	unverifiedReasons := []string{"Completed", "Implicit", "Pending"}
	for _, faUser := range faUsers {
		var identity *FaIdentity
		for _, i := range faUser.Identities {
			if i.Primary {
				identity = &i
				break
			}
		}
		var registration *FaRegistration
		for _, r := range faUser.Registrations {
			if r.ApplicationId == applicationId {
				registration = &r
				break
			}
		}
		if identity == nil || registration == nil {
			continue
		}
		user := UserOutput{
			Id:             faUser.Id,
			Email:          faUser.Email,
			IsVerified:     identity.Verified || !contains(unverifiedReasons, identity.VerifiedReason),
			RegisteredDate: registration.InsertInstant,
			LoginDates:     []int64{},
		}
		var loginResp FaLoginResponse
		getFaData("/api/system/login-record/search?userId="+user.Id+"&startRow=0&numberOfResults=999999", &loginResp)
		for _, l := range loginResp.Logins {
			user.LoginDates = append(user.LoginDates, l.Instant)
		}
		sort.Slice(user.LoginDates, func(i, j int) bool { return user.LoginDates[i] < user.LoginDates[j] })
		checkDates(user, *registration, loginResp)
		users = append(users, user)
	}
	return users
}

func getFaData(url string, target interface{}) {
	client := &http.Client{}
	request, _ := http.NewRequest("GET", faUrl+url, nil)
	request.Header.Set("Authorization", apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		fmt.Printf("httpError! status: %d\n", response.StatusCode)
		return
	}
	body, _ := io.ReadAll(response.Body)
	json.Unmarshal(body, target)
}

func checkDates(user UserOutput, registration FaRegistration, logins FaLoginResponse) {
	if len(fmt.Sprintf("%d", registration.InsertInstant)) != 13 {
		fmt.Println("Date error: FusionAuth returned registration timestamp that doesn't have 13 digits:")
		fmt.Printf("%+v\n", user)
		os.Exit(1)
	}
	for _, l := range logins.Logins {
		if len(fmt.Sprintf("%d", l.Instant)) != 13 {
			fmt.Println("Date error: FusionAuth returned login timestamp that doesn't have 13 digits:")
			fmt.Printf("%+v\n", user)
			os.Exit(1)
		}
	}
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

type FaIdentity struct {
	Primary        bool   `json:"primary"`
	Verified       bool   `json:"verified"`
	VerifiedReason string `json:"verifiedReason"`
}

type FaRegistration struct {
	ApplicationId string `json:"applicationId"`
	InsertInstant int64  `json:"insertInstant"`
}

type FaUser struct {
	Id            string           `json:"id"`
	Email         string           `json:"email"`
	Identities    []FaIdentity     `json:"identities"`
	Registrations []FaRegistration `json:"registrations"`
}

type FaUserResponse struct {
	Users []FaUser `json:"users"`
}

type FaLogin struct {
	Instant int64 `json:"instant"`
}

type FaLoginResponse struct {
	Logins []FaLogin `json:"logins"`
}

type UserOutput struct {
	Id             string  `json:"id"`
	Email          string  `json:"email"`
	IsVerified     bool    `json:"isVerified"`
	RegisteredDate int64   `json:"registeredDate"`
	LoginDates     []int64 `json:"loginDates"`
}
