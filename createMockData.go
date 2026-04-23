// docker run --init  -it  --rm --platform linux/amd64 --name "app" --network faNetwork -v .:/app -v ./gocache:/go/pkg -v ./buildcache:/root/.cache/go-build -w /app golang:1.25-bookworm sh -c "go fmt createMockData.go && go run createMockData.go"

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

var fusionauthUrl = os.Getenv("FUSIONAUTH_URL")
var apiKey = os.Getenv("API_KEY")
var applicationId = os.Getenv("APPLICATION_ID")
const numberOfUsersToCreate = 1000

func main() {
	client := &http.Client{}
	registrationUrl := fusionauthUrl + "/api/user/registration"
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
		request, _ := http.NewRequest("POST", registrationUrl, bytes.NewBuffer(jsonData))
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
