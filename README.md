# FusionAuth User Charts

A pipeline that creates mock FusionAuth user data, extracts it via the FusionAuth API, and displays user analytics charts in a browser. All scripts are numbered in execution order and run in Docker containers on a shared `faNetwork` with a FusionAuth instance.

## Scripts

### 1createMockData.go
Registers 1000 mock users against the FusionAuth API with sequential email addresses (`1@example.com`, `2@example.com`, etc.).

### 2createMockData.sql
Runs directly against the FusionAuth PostgreSQL database to make the mock data more realistic:
- Randomizes registration dates between 2015 and 2025.
- Sets 5% of users as unverified.
- Generates random login records distributed between each user's registration date and end of 2025.
- Removes logins for unverified users.

### 3extract.go
Queries the FusionAuth API to fetch all users and their login records, then writes two JSON files:
- `faUsers.json` — raw API response.
- `users.json` — simplified extract with id, email, verification status, registration date, and sorted login dates.

### 4app.go
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

### 5page.html
Single-page dashboard rendered with Chart.js. Displays all 16 charts using data injected by `4app.go`. Includes a retention heatmap via the `chartjs-chart-matrix` plugin.

## Running

Each script includes a Docker run command in its first comment line. Execute them in order (1–4) against a running FusionAuth instance on Docker network `faNetwork`.
