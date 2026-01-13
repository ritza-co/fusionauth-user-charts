// docker run --init  -it  --rm --platform linux/amd64 --name "app" --network faNetwork -v .:/app -v ./gocache:/go/pkg -v ./buildcache:/root/.cache/go-build -w /app golang:1.25-bookworm sh -c "go fmt 1createMockData.go && go run 1createMockData.go"

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const fusionauthUrl = "http://fa:9011/api/user/registration"
const apiKey = "33052c8a-c283-4e96-9d2a-eb1215c69f8f-not-for-prod"
const applicationId = "e9fdb985-9173-4e01-9d73-ac2d60d1dc8e"
const numberOfUsersToCreate = 1000

func main() {
	client := &http.Client{}
	for i := 1; i <= numberOfUsersToCreate; i++ {
		email := fmt.Sprintf("%d@example.com", i)
		requestBody := RegistrationRequest{
			User: UserDetail{
				Email:    email,
				Password: "password",
			},
			Registration: ApplicationDetail{
				ApplicationId: applicationId,
			},
		}
		jsonData, _ := json.Marshal(requestBody)
		request, _ := http.NewRequest("POST", fusionauthUrl, bytes.NewBuffer(jsonData))
		request.Header.Set("Authorization", apiKey)
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			fmt.Printf("networkError for user %s: %s\n", email, err.Error())
			return
		}
		defer response.Body.Close()
		responseBody, _ := io.ReadAll(response.Body)
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			fmt.Printf("httpError %d for user %s: %s\n", response.StatusCode, email, string(responseBody))
			return
		}
		fmt.Println(string(responseBody))
		fmt.Println("")
	}
}

type RegistrationRequest struct {
	User         UserDetail        `json:"user"`
	Registration ApplicationDetail `json:"registration"`
}

type UserDetail struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ApplicationDetail struct {
	ApplicationId string `json:"applicationId"`
}
