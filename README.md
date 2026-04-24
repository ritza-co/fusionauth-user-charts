# FusionAuth User Charts

A pipeline that creates mock FusionAuth user data, extracts it via the FusionAuth API, and displays user analytics charts in a browser.

## Running

Run the scripts with Go in the order shown below. The second script needs to be run with PostgreSQL.


### createMockData.go
Registers 1000 mock users against the FusionAuth API with sequential email addresses (`1@example.com`, `2@example.com`, etc.).

### createMockData.sql
Runs directly against the FusionAuth PostgreSQL database to make the mock data more realistic:
- Randomizes registration dates between 2015 and 2025.
- Sets 5% of users as unverified.
- Generates random login records distributed between each user's registration date and end of 2025.
- Removes logins for unverified users.

### extractUsers.go
Queries the FusionAuth API to fetch all users and their login records, then writes two JSON files:
- `faUsers.json` — raw API response.
- `users.json` — simplified extract with id, email, verification status, registration date, and sorted login dates.

### charts.go
Reads `users.json` and computes 16 chart datasets covering:
- Total and new users (yearly/monthly), split by verified/unverified.
- User account age distribution.
- Login counts and login-to-user ratios (yearly/monthly).
- Abandonment (users inactive for 1/2/6/12 months).
- Users inactive for 6+ months per year.
- Activity cohorts (0 / ≤4 / >4 logins in the past year).
- Returning users (back after 1+ year absent).
- Cohort retention heatmap (months 0–12 after registration).
- Friction (time from registration to first login).
- Login frequency (unique login days in the past 30 days).

Serves the results as an HTML page on port 7777.
