package main

import (
	"reflect"
	"testing"
	"time"
)

func TestCalculateReturningUsersChart_LateLabelDoesNotWipeEarlierCounts(t *testing.T) {
	mustDate := func(s string) time.Time {
		ts, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatalf("bad date %q: %v", s, err)
		}
		return ts
	}

	// Three users who each return after more than a year of inactivity.
	// Alice and Bob return in March 2025; Carol returns in January 2025.
	// Carol is processed last so her "January" label is discovered after
	// "March" has already accumulated counts — this is exactly the case
	// that the original implementation got wrong by reallocating its
	// data slices when a new label appeared.
	users := []User{
		{
			IsVerified:     true,
			RegisteredDate: mustDate("2023-01-01"),
			LoginDates:     []time.Time{mustDate("2025-03-15")},
		},
		{
			IsVerified:     true,
			RegisteredDate: mustDate("2023-01-01"),
			LoginDates:     []time.Time{mustDate("2025-03-20")},
		},
		{
			IsVerified:     true,
			RegisteredDate: mustDate("2023-01-01"),
			LoginDates:     []time.Time{mustDate("2025-01-10")},
		},
	}

	got := calculateReturningUsersChart(users)

	wantLabels := []string{"2025-01", "2025-03"}
	wantVerified := []int{1, 2}
	wantUnverified := []int{0, 0}

	if !reflect.DeepEqual(got.Labels, wantLabels) {
		t.Errorf("Labels: got %v, want %v", got.Labels, wantLabels)
	}
	if !reflect.DeepEqual(got.VerifiedData, wantVerified) {
		t.Errorf("VerifiedData: got %v, want %v", got.VerifiedData, wantVerified)
	}
	if !reflect.DeepEqual(got.UnverifiedData, wantUnverified) {
		t.Errorf("UnverifiedData: got %v, want %v", got.UnverifiedData, wantUnverified)
	}
}
