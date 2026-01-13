// docker run --init  -it  --rm --platform linux/amd64 --name "app" -p 7777:7777 --network faNetwork -v .:/app -v ./gocache:/go/pkg -v ./buildcache:/root/.cache/go-build -w /app golang:1.25-bookworm sh -c "go mod tidy && go fmt 4app.go && go run 4app.go"

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
)

func main() {
	users := getUsersFromFile()
	chartData := getChartData(users)
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		page := getPage(chartData)
		fmt.Fprint(writer, page)
	})
	fmt.Println("Server listening at http://0.0.0.0:7777")
	http.ListenAndServe("0.0.0.0:7777", nil)
}

func getUsersFromFile() []User {
	fileContent, _ := os.ReadFile("users.json")
	var users []User
	json.Unmarshal(fileContent, &users)
	for userIndex := range users {
		user := &users[userIndex]
		user.LoginDatesUniqueMonthly = []time.Time{}
		user.LoginDatesUniqueYearly = []time.Time{}
		registrationString := fmt.Sprintf("%d", user.RegisteredDateRaw)
		registrationError := len(registrationString) != 13
		user.RegisteredDate = time.UnixMilli(user.RegisteredDateRaw)
		for _, timestamp := range user.LoginDatesRaw {
			timestampString := fmt.Sprintf("%d", timestamp)
			if registrationError || len(timestampString) != 13 {
				fmt.Println(" ")
				fmt.Println("Date error: FusionAuth returned timestamp that doesn't have 13 digits:")
				fmt.Println(user.Id)
				fmt.Println(user.Email)
				fmt.Println(user.RegisteredDate)
				fmt.Println(timestamp)
				fmt.Println(time.UnixMilli(timestamp))
				os.Exit(1)
			}
			user.LoginDates = append(user.LoginDates, time.UnixMilli(timestamp))
		}
	}
	addDeduplicatedLoginDates(users)
	return users
}

func addDeduplicatedLoginDates(users []User) {
	for userIndex := range users {
		user := &users[userIndex]
		for _, loginDate := range user.LoginDates {
			isYearPresent := false
			for _, yearlyDate := range user.LoginDatesUniqueYearly {
				if yearlyDate.Year() == loginDate.Year() {
					isYearPresent = true
					break
				}
			}
			if !isYearPresent {
				user.LoginDatesUniqueYearly = append(user.LoginDatesUniqueYearly, loginDate)
			}
			isMonthPresent := false
			for _, monthlyDate := range user.LoginDatesUniqueMonthly {
				if monthlyDate.Year() == loginDate.Year() && monthlyDate.Month() == loginDate.Month() {
					isMonthPresent = true
					break
				}
			}
			if !isMonthPresent {
				user.LoginDatesUniqueMonthly = append(user.LoginDatesUniqueMonthly, loginDate)
			}
		}
	}
}

func getPage(chartData ChartResult) string {
	htmlBytes, _ := os.ReadFile("5page.html")
	html := string(htmlBytes)
	chartJson, _ := json.Marshal(chartData)
	return strings.Replace(html, "{{CHARTDATA}}", string(chartJson), 1)
}

func getChartData(users []User) ChartResult {
	now := time.Now()
	thisYear := now.Year()
	minYear := getMinYear(users, thisYear)
	maxYear := getMaxYear(users, thisYear)
	startYear, startMonth := getMinYearAndMonth(users, thisYear)
	result := ChartResult{
		TotalUsersPerYearChart:        ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}},
		TotalUsersPerMonthChart:       ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}},
		NewUsersPerYearChart:          ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}},
		NewUsersPerMonthChart:         ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}},
		UserAgeChart:                  ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}},
		LoginsPerYearChart:            SimpleChartData{Labels: []string{}, Data: []int{}},
		LoginsPerMonthChart:           SimpleChartData{Labels: []string{}, Data: []int{}},
		PercentLoginsPerYearChart:     SimpleChartFloatData{Labels: []string{}, Data: []float64{}},
		PercentLoginsPerMonthChart:    SimpleChartFloatData{Labels: []string{}, Data: []float64{}},
		AbandonmentPerMonthChart:      ChartData{Labels: []string{"1", "2", "6", "12"}, VerifiedData: []int{0, 0, 0, 0}, UnverifiedData: []int{0, 0, 0, 0}},
		InactiveSixMonthsPerYearChart: ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}},
		ActivityCohortChart:           ChartData{Labels: []string{"0", "<= 4", "> 4"}, VerifiedData: []int{0, 0, 0}, UnverifiedData: []int{0, 0, 0}},
		ReturningUsersChart:           ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}},
		RetentionChart:                RetentionChartData{},
		FrictionChart:                 ChartData{Labels: []string{"< 1 day", "< 1 week", "< 1 month", "> 1 month", "Never"}, VerifiedData: []int{0, 0, 0, 0, 0}, UnverifiedData: []int{0, 0, 0, 0, 0}},
		LoginFrequencyChart:           ChartData{Labels: []string{}, VerifiedData: make([]int, 32), UnverifiedData: make([]int, 32)},
	}
	var waitGroup sync.WaitGroup
	var mutex sync.Mutex
	runParallel(&waitGroup, &mutex, &result.TotalUsersPerYearChart, func() any { return calculateTotalUsersPerYearChart(users, minYear, maxYear) })
	runParallel(&waitGroup, &mutex, &result.TotalUsersPerMonthChart, func() any {
		return calculateTotalUsersPerMonthChart(users, startYear, maxYear, startMonth, now.Month())
	})
	runParallel(&waitGroup, &mutex, &result.NewUsersPerYearChart, func() any { return calculateNewUsersPerYearChart(users, minYear, maxYear) })
	runParallel(&waitGroup, &mutex, &result.NewUsersPerMonthChart, func() any { return calculateNewUsersPerMonthChart(users, startYear, maxYear, startMonth, now.Month()) })
	runParallel(&waitGroup, &mutex, &result.UserAgeChart, func() any { return calculateUserAgeChart(users, thisYear, minYear) })
	runParallel(&waitGroup, &mutex, &result.LoginsPerYearChart, func() any { return calculateLoginsPerYearChart(users, minYear, maxYear) })
	runParallel(&waitGroup, &mutex, &result.LoginsPerMonthChart, func() any { return calculateLoginsPerMonthChart(users, startYear, maxYear, startMonth, now.Month()) })
	runParallel(&waitGroup, &mutex, &result.PercentLoginsPerYearChart, func() any { return calculatePercentLoginsPerYearChart(users, minYear, maxYear) })
	runParallel(&waitGroup, &mutex, &result.PercentLoginsPerMonthChart, func() any {
		return calculatePercentLoginsPerMonthChart(users, startYear, maxYear, startMonth, now.Month())
	})
	runParallel(&waitGroup, &mutex, &result.AbandonmentPerMonthChart, func() any { return calculateAbandonmentPerMonthChart(users, now) })
	runParallel(&waitGroup, &mutex, &result.InactiveSixMonthsPerYearChart, func() any { return calculateInactiveSixMonthsPerYearChart(users, minYear, maxYear, now) })
	runParallel(&waitGroup, &mutex, &result.ActivityCohortChart, func() any { return calculateActivityCohortChart(users) })
	runParallel(&waitGroup, &mutex, &result.ReturningUsersChart, func() any { return calculateReturningUsersChart(users) })
	runParallel(&waitGroup, &mutex, &result.RetentionChart, func() any { return calculateRetentionChart(users) })
	runParallel(&waitGroup, &mutex, &result.FrictionChart, func() any { return calculateFrictionChart(users) })
	runParallel(&waitGroup, &mutex, &result.LoginFrequencyChart, func() any { return calculateLoginFrequencyChart(users, now) })
	waitGroup.Wait()
	return result
}

func runParallel[T any](waitGroup *sync.WaitGroup, mutex *sync.Mutex, target *T, task func() any) {
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		val := task().(T)
		mutex.Lock()
		*target = val
		mutex.Unlock()
	}()
}

func calculateTotalUsersPerYearChart(users []User, minYear int, maxYear int) ChartData {
	chart := ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}}
	runningVerified, runningUnverified := 0, 0
	for year := minYear; year <= maxYear; year++ {
		verifiedThisYear, unverifiedThisYear := getRegistrationCounts(users, year, 0, false)
		runningVerified += verifiedThisYear
		runningUnverified += unverifiedThisYear
		chart.Labels = append(chart.Labels, fmt.Sprintf("%d", year))
		chart.VerifiedData = append(chart.VerifiedData, runningVerified)
		chart.UnverifiedData = append(chart.UnverifiedData, runningUnverified)
	}
	return chart
}

func calculateTotalUsersPerMonthChart(users []User, startYear int, maxYear int, startMonth time.Month, currentMonth time.Month) ChartData {
	chart := ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}}
	runningVerified, runningUnverified := 0, 0
	forEachMonth(startYear, maxYear, startMonth, currentMonth, func(year int, month int) {
		verifiedThisMonth, unverifiedThisMonth := getRegistrationCounts(users, year, month, true)
		runningVerified += verifiedThisMonth
		runningUnverified += unverifiedThisMonth
		chart.Labels = append(chart.Labels, fmt.Sprintf("%d-%02d", year, month))
		chart.VerifiedData = append(chart.VerifiedData, runningVerified)
		chart.UnverifiedData = append(chart.UnverifiedData, runningUnverified)
	})
	return chart
}

func calculateNewUsersPerYearChart(users []User, minYear int, maxYear int) ChartData {
	chart := ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}}
	for year := minYear; year <= maxYear; year++ {
		verifiedThisYear, unverifiedThisYear := getRegistrationCounts(users, year, 0, false)
		chart.Labels = append(chart.Labels, fmt.Sprintf("%d", year))
		chart.VerifiedData = append(chart.VerifiedData, verifiedThisYear)
		chart.UnverifiedData = append(chart.UnverifiedData, unverifiedThisYear)
	}
	return chart
}

func calculateNewUsersPerMonthChart(users []User, startYear int, maxYear int, startMonth time.Month, currentMonth time.Month) ChartData {
	chart := ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}}
	forEachMonth(startYear, maxYear, startMonth, currentMonth, func(year int, month int) {
		verifiedThisMonth, unverifiedThisMonth := getRegistrationCounts(users, year, month, true)
		chart.Labels = append(chart.Labels, fmt.Sprintf("%d-%02d", year, month))
		chart.VerifiedData = append(chart.VerifiedData, verifiedThisMonth)
		chart.UnverifiedData = append(chart.UnverifiedData, unverifiedThisMonth)
	})
	return chart
}

func calculateUserAgeChart(users []User, thisYear int, minYear int) ChartData {
	maxAge := thisYear - minYear
	chart := ChartData{Labels: []string{}, VerifiedData: make([]int, maxAge+1), UnverifiedData: make([]int, maxAge+1)}
	for _, user := range users {
		age := thisYear - user.RegisteredDate.Year()
		if age >= 0 && age <= maxAge {
			incrementChartData(&chart, user.IsVerified, age)
		}
	}
	for age := 0; age <= maxAge; age++ {
		chart.Labels = append(chart.Labels, fmt.Sprintf("%d", age))
	}
	return chart
}

func calculateLoginsPerYearChart(users []User, minYear int, maxYear int) SimpleChartData {
	chart := SimpleChartData{Labels: []string{}, Data: []int{}}
	for year := minYear; year <= maxYear; year++ {
		count := getLoginCount(users, year, 0, false)
		chart.Labels = append(chart.Labels, fmt.Sprintf("%d", year))
		chart.Data = append(chart.Data, count)
	}
	return chart
}

func calculateLoginsPerMonthChart(users []User, startYear int, maxYear int, startMonth time.Month, currentMonth time.Month) SimpleChartData {
	chart := SimpleChartData{Labels: []string{}, Data: []int{}}
	forEachMonth(startYear, maxYear, startMonth, currentMonth, func(year int, month int) {
		count := getLoginCount(users, year, month, true)
		chart.Labels = append(chart.Labels, fmt.Sprintf("%d-%02d", year, month))
		chart.Data = append(chart.Data, count)
	})
	return chart
}

func calculatePercentLoginsPerYearChart(users []User, minYear int, maxYear int) SimpleChartFloatData {
	chart := SimpleChartFloatData{Labels: []string{}, Data: []float64{}}
	runningVerified, runningUnverified := 0, 0
	for year := minYear; year <= maxYear; year++ {
		verifiedThisYear, unverifiedThisYear := getRegistrationCounts(users, year, 0, false)
		loginCount := getLoginCount(users, year, 0, false)
		runningVerified += verifiedThisYear
		runningUnverified += unverifiedThisYear
		totalUsers := runningVerified + runningUnverified
		chart.Labels = append(chart.Labels, fmt.Sprintf("%d", year))
		chart.Data = append(chart.Data, calculateRatio(loginCount, totalUsers))
	}
	return chart
}

func calculatePercentLoginsPerMonthChart(users []User, startYear int, maxYear int, startMonth time.Month, currentMonth time.Month) SimpleChartFloatData {
	chart := SimpleChartFloatData{Labels: []string{}, Data: []float64{}}
	runningVerified, runningUnverified := 0, 0
	forEachMonth(startYear, maxYear, startMonth, currentMonth, func(year int, month int) {
		verifiedThisMonth, unverifiedThisMonth := getRegistrationCounts(users, year, month, true)
		loginCount := getLoginCount(users, year, month, true)
		runningVerified += verifiedThisMonth
		runningUnverified += unverifiedThisMonth
		totalUsers := runningVerified + runningUnverified
		chart.Labels = append(chart.Labels, fmt.Sprintf("%d-%02d", year, month))
		chart.Data = append(chart.Data, calculateRatio(loginCount, totalUsers))
	})
	return chart
}

func calculateAbandonmentPerMonthChart(users []User, now time.Time) ChartData {
	chart := ChartData{Labels: []string{"1", "2", "6", "12"}, VerifiedData: make([]int, 4), UnverifiedData: make([]int, 4)}
	for _, user := range users {
		if len(user.LoginDates) == 0 {
			continue
		}
		mostRecent := user.LoginDates[len(user.LoginDates)-1]
		differenceDays := int(now.Sub(mostRecent).Hours() / 24)
		index := -1
		if differenceDays >= 365 {
			index = 3
		} else if differenceDays >= 182 {
			index = 2
		} else if differenceDays >= 60 {
			index = 1
		} else if differenceDays >= 30 {
			index = 0
		}
		if index != -1 {
			incrementChartData(&chart, user.IsVerified, index)
		}
	}
	return chart
}

func calculateInactiveSixMonthsPerYearChart(users []User, minYear int, maxYear int, now time.Time) ChartData {
	chart := ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}}
	sixMonthDuration := 6 * 30 * 24 * time.Hour
	for year := minYear; year <= maxYear; year++ {
		verifiedLapsedCount, unverifiedLapsedCount := 0, 0
		yearStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		yearEnd := time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC)
		observationEnd := yearEnd
		if now.Before(yearEnd) {
			observationEnd = now
		}
		for _, user := range users {
			if user.RegisteredDate.After(yearEnd) {
				continue
			}
			userEventTimeline := append([]time.Time{user.RegisteredDate}, lo.Filter(user.LoginDates, func(loginTimestamp time.Time, _ int) bool {
				return loginTimestamp.Before(observationEnd) || loginTimestamp.Equal(observationEnd)
			})...)
			userEventTimeline = append(userEventTimeline, observationEnd)
			isUserCurrentlyLapsed := false
			for i := 1; i < len(userEventTimeline); i++ {
				inactivityGap := userEventTimeline[i].Sub(userEventTimeline[i-1])
				timestampOfLapse := userEventTimeline[i-1].Add(sixMonthDuration)
				if inactivityGap >= sixMonthDuration && (timestampOfLapse.After(yearStart) || timestampOfLapse.Equal(yearStart)) && (timestampOfLapse.Before(yearEnd) || timestampOfLapse.Equal(yearEnd)) {
					isUserCurrentlyLapsed = true
					break
				}
			}
			if isUserCurrentlyLapsed {
				if user.IsVerified {
					verifiedLapsedCount++
				} else {
					unverifiedLapsedCount++
				}
			}
		}
		chart.Labels = append(chart.Labels, fmt.Sprintf("%d", year))
		chart.VerifiedData = append(chart.VerifiedData, verifiedLapsedCount)
		chart.UnverifiedData = append(chart.UnverifiedData, unverifiedLapsedCount)
	}
	return chart
}

func calculateActivityCohortChart(users []User) ChartData {
	chart := ChartData{Labels: []string{"0", "<= 4", "> 4"}, VerifiedData: make([]int, 3), UnverifiedData: make([]int, 3)}
	now := time.Now()
	oneYearAgo := now.AddDate(-1, 0, 0)
	for _, user := range users {
		loginsInPastYearCount := len(lo.Filter(user.LoginDates, func(loginTimestamp time.Time, _ int) bool {
			return loginTimestamp.After(oneYearAgo) || loginTimestamp.Equal(oneYearAgo)
		}))
		cohortIndex := 0
		if loginsInPastYearCount > 4 {
			cohortIndex = 2
		} else if loginsInPastYearCount > 0 {
			cohortIndex = 1
		}
		incrementChartData(&chart, user.IsVerified, cohortIndex)
	}
	return chart
}

func calculateReturningUsersChart(users []User) ChartData {
	chart := ChartData{Labels: []string{}, VerifiedData: []int{}, UnverifiedData: []int{}}
	labelMap := make(map[string]int)
	for _, user := range users {
		previous := user.RegisteredDate
		for _, login := range user.LoginDates {
			if login.Sub(previous).Hours() > 24*365 {
				label := login.Format("2006-01")
				if _, exists := labelMap[label]; !exists {
					chart.Labels = append(chart.Labels, label)
					sort.Strings(chart.Labels)
					chart.VerifiedData = make([]int, len(chart.Labels))
					chart.UnverifiedData = make([]int, len(chart.Labels))
					for index, labelValue := range chart.Labels {
						labelMap[labelValue] = index
					}
				}
				index := labelMap[label]
				incrementChartData(&chart, user.IsVerified, index)
			}
			previous = login
		}
	}
	return chart
}

func calculateRetentionChart(users []User) RetentionChartData {
	maxMonths := 12
	cohorts := make(map[string]*CohortData)
	for _, user := range users {
		key := user.RegisteredDate.Format("2006-01")
		if _, exists := cohorts[key]; !exists {
			cohorts[key] = &CohortData{Counts: make([]int, maxMonths+1)}
		}
		cohorts[key].TotalUsers++
		activeMonths := make(map[int]bool)
		for _, login := range user.LoginDates {
			difference := (login.Year()-user.RegisteredDate.Year())*12 + int(login.Month()-user.RegisteredDate.Month())
			if difference >= 0 && difference <= maxMonths {
				activeMonths[difference] = true
			}
		}
		for month := range activeMonths {
			cohorts[key].Counts[month]++
		}
	}
	keys := lo.Keys(cohorts)
	sort.Strings(keys)
	chart := RetentionChartData{XLabels: keys, YLabels: []string{}, MatrixData: []MatrixPoint{}}
	for month := 0; month <= maxMonths; month++ {
		chart.YLabels = append(chart.YLabels, fmt.Sprintf("%d", month))
	}
	for _, key := range keys {
		cohort := cohorts[key]
		for month, count := range cohort.Counts {
			value := 0.0
			if cohort.TotalUsers > 0 {
				value = math.Round(float64(count)*1000/float64(cohort.TotalUsers)) / 10
			}
			chart.MatrixData = append(chart.MatrixData, MatrixPoint{X: key, Y: fmt.Sprintf("%d", month), V: value})
		}
	}
	return chart
}

func calculateFrictionChart(users []User) ChartData {
	chart := ChartData{Labels: []string{"< 1 day", "< 1 week", "< 1 month", "> 1 month", "Never"}, VerifiedData: make([]int, 5), UnverifiedData: make([]int, 5)}
	day, week, month := 24*time.Hour, 7*24*time.Hour, 30*24*time.Hour
	for _, user := range users {
		if len(user.LoginDatesUniqueMonthly) == 0 {
			incrementChartData(&chart, user.IsVerified, 4)
			continue
		}
		difference := user.LoginDatesUniqueMonthly[0].Sub(user.RegisteredDate)
		index := 3
		if difference <= day {
			index = 0
		} else if difference <= week {
			index = 1
		} else if difference <= month {
			index = 2
		}
		incrementChartData(&chart, user.IsVerified, index)
	}
	return chart
}

func calculateLoginFrequencyChart(users []User, now time.Time) ChartData {
	chart := ChartData{Labels: make([]string, 32), VerifiedData: make([]int, 32), UnverifiedData: make([]int, 32)}
	for i := 0; i <= 31; i++ {
		chart.Labels[i] = fmt.Sprintf("%d", i)
	}
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)
	for _, user := range users {
		uniqueDays := make(map[string]bool)
		for _, login := range user.LoginDates {
			if (login.After(thirtyDaysAgo) || login.Equal(thirtyDaysAgo)) && (login.Before(now) || login.Equal(now)) {
				uniqueDays[login.Format("2006-01-02")] = true
			}
		}
		count := len(uniqueDays)
		if count > 31 {
			count = 31
		}
		incrementChartData(&chart, user.IsVerified, count)
	}
	return chart
}

func getMinYear(users []User, currentYear int) int {
	min := currentYear
	for _, user := range users {
		if user.RegisteredDate.Year() < min {
			min = user.RegisteredDate.Year()
		}
	}
	return min
}

func getMaxYear(users []User, currentYear int) int {
	max := 0
	for _, user := range users {
		if user.RegisteredDate.Year() > max {
			max = user.RegisteredDate.Year()
		}
		for _, login := range user.LoginDates {
			if login.Year() > max {
				max = login.Year()
			}
		}
	}
	return max
}

func getMinYearAndMonth(users []User, currentYear int) (int, time.Month) {
	minYear, minMonth := currentYear, time.Month(12)
	for _, user := range users {
		if user.RegisteredDate.Year() < minYear || (user.RegisteredDate.Year() == minYear && user.RegisteredDate.Month() < minMonth) {
			minYear, minMonth = user.RegisteredDate.Year(), user.RegisteredDate.Month()
		}
	}
	return minYear, minMonth
}

func forEachMonth(startYear int, maxYear int, startMonth time.Month, currentMonth time.Month, action func(year int, month int)) {
	for year := startYear; year <= maxYear; year++ {
		for month := 1; month <= 12; month++ {
			if year == startYear && time.Month(month) < startMonth {
				continue
			}
			if year == maxYear && time.Month(month) > currentMonth {
				break
			}
			action(year, month)
		}
	}
}

func incrementChartData(chart *ChartData, isVerified bool, index int) {
	if isVerified {
		chart.VerifiedData[index]++
	} else {
		chart.UnverifiedData[index]++
	}
}

func calculateRatio(numerator int, denominator int) float64 {
	if denominator <= 0 {
		return 0.0
	}
	return float64(numerator) / float64(denominator)
}

func getRegistrationCounts(users []User, year int, month int, useMonthly bool) (int, int) {
	verified := len(lo.Filter(users, func(user User, _ int) bool {
		match := user.RegisteredDate.Year() == year
		if useMonthly {
			match = match && int(user.RegisteredDate.Month()) == month
		}
		return match && user.IsVerified
	}))
	unverified := len(lo.Filter(users, func(user User, _ int) bool {
		match := user.RegisteredDate.Year() == year
		if useMonthly {
			match = match && int(user.RegisteredDate.Month()) == month
		}
		return match && !user.IsVerified
	}))
	return verified, unverified
}

func getLoginCount(users []User, year int, month int, useMonthly bool) int {
	return lo.Reduce(users, func(accumulator int, user User, _ int) int {
		dates := user.LoginDatesUniqueYearly
		if useMonthly {
			dates = user.LoginDatesUniqueMonthly
		}
		return accumulator + len(lo.Filter(dates, func(timestamp time.Time, _ int) bool {
			match := timestamp.Year() == year
			if useMonthly {
				match = match && int(timestamp.Month()) == month
			}
			return match
		}))
	}, 0)
}

type User struct {
	Id                      string      `json:"id"`
	Email                   string      `json:"email"`
	IsVerified              bool        `json:"isVerified"`
	RegisteredDateRaw       int64       `json:"registeredDate"`
	RegisteredDate          time.Time   `json:"-"`
	LoginDatesRaw           []int64     `json:"loginDates"`
	LoginDates              []time.Time `json:"-"`
	LoginDatesUniqueMonthly []time.Time `json:"-"`
	LoginDatesUniqueYearly  []time.Time `json:"-"`
}

type ChartData struct {
	Labels         []string `json:"labels"`
	VerifiedData   []int    `json:"verifiedData"`
	UnverifiedData []int    `json:"unverifiedData"`
}

type SimpleChartData struct {
	Labels []string `json:"labels"`
	Data   []int    `json:"data"`
}

type SimpleChartFloatData struct {
	Labels []string  `json:"labels"`
	Data   []float64 `json:"data"`
}

type MatrixPoint struct {
	X string  `json:"x"`
	Y string  `json:"y"`
	V float64 `json:"v"`
}

type RetentionChartData struct {
	XLabels    []string      `json:"xLabels"`
	YLabels    []string      `json:"yLabels"`
	MatrixData []MatrixPoint `json:"matrixData"`
}

type CohortData struct {
	TotalUsers int
	Counts     []int
}

type ChartResult struct {
	TotalUsersPerYearChart        ChartData            `json:"totalUsersPerYearChart"`
	TotalUsersPerMonthChart       ChartData            `json:"totalUsersPerMonthChart"`
	NewUsersPerYearChart          ChartData            `json:"newUsersPerYearChart"`
	NewUsersPerMonthChart         ChartData            `json:"newUsersPerMonthChart"`
	UserAgeChart                  ChartData            `json:"userAgeChart"`
	LoginsPerYearChart            SimpleChartData      `json:"loginsPerYearChart"`
	LoginsPerMonthChart           SimpleChartData      `json:"loginsPerMonthChart"`
	PercentLoginsPerYearChart     SimpleChartFloatData `json:"percentLoginsPerYearChart"`
	PercentLoginsPerMonthChart    SimpleChartFloatData `json:"percentLoginsPerMonthChart"`
	AbandonmentPerMonthChart      ChartData            `json:"abandonmentPerMonthChart"`
	InactiveSixMonthsPerYearChart ChartData            `json:"inactiveSixMonthsPerYearChart"`
	ActivityCohortChart           ChartData            `json:"activityCohortChart"`
	ReturningUsersChart           ChartData            `json:"returningUsersChart"`
	RetentionChart                RetentionChartData   `json:"retentionChart"`
	FrictionChart                 ChartData            `json:"frictionChart"`
	LoginFrequencyChart           ChartData            `json:"loginFrequencyChart"`
}
