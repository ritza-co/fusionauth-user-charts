package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/FusionAuth/go-client/pkg/fusionauth"
	"github.com/joho/godotenv"
)

const numberOfUsersToCreate = 20000

func main() {
	godotenv.Load()
	var fusionauthUrl = os.Getenv("FUSIONAUTH_URL")
	var apiKey = os.Getenv("API_KEY")
	var applicationId = os.Getenv("APPLICATION_ID")
	baseURL, _ := url.Parse(fusionauthUrl)
	client := fusionauth.NewClient(http.DefaultClient, baseURL, apiKey)
	const batchSize = 100
	startTime := time.Now()
	for i := 0; i < numberOfUsersToCreate; i += batchSize {
		var users []fusionauth.User
		for j := 0; j < batchSize && (i+j) < numberOfUsersToCreate; j++ {
			userNum := i + j + 1
			email := fmt.Sprintf("%d@example.com", userNum)
			u := fusionauth.User{
				Email: email,
				Registrations: []fusionauth.UserRegistration{
					{
						ApplicationId: applicationId,
					},
				},
			}
			u.Password = "password"
			users = append(users, u)
		}
		_, errors, err := client.ImportUsers(fusionauth.ImportRequest{
			Users:                 users,
			ValidateDbConstraints: true,
		})
		if err != nil {
			fmt.Printf("Batch error starting at %d: %s\n", i, err.Error())
			return
		}
		if errors != nil {
			fmt.Printf("HTTP Error in batch starting at %d: %v\n", i, errors)
			return
		}
		fmt.Printf("Registered (imported) user batch: %d of %d\n", i+len(users), numberOfUsersToCreate)
	}
	duration := time.Since(startTime)
	fmt.Printf("Total time: %s\n", duration)
	fmt.Printf("Time per user: %s\n", duration/time.Duration(numberOfUsersToCreate))
}
