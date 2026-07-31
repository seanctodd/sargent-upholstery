package main

import "testing"

func rev(id string, rating int, text string) Review {
	return Review{ID: id, Author: "A", Rating: rating, Text: text, Date: "2026-01-01"}
}

// long enough to clear minWords (20)
const longText = "one two three four five six seven eight nine ten eleven twelve " +
	"thirteen fourteen fifteen sixteen seventeen eighteen nineteen twenty twentyone"

const shortText = "too short"

func TestNoChangeWhenAllPresentAndQualifying(t *testing.T) {
	existing := []Review{rev("a", 5, longText), rev("b", 5, longText)}
	fetched := []Review{rev("a", 5, longText), rev("b", 5, longText)}
	out, tomb, skipped := reconcile(existing, fetched, "2026-07-31", 5)
	if skipped || len(tomb) != 0 {
		t.Fatalf("expected no removals, got %d skipped=%v", len(tomb), skipped)
	}
	for _, r := range out {
		if r.Removed != "" {
			t.Fatalf("review %s wrongly tombstoned", r.ID)
		}
	}
}

func TestAbsentReviewIsTombstoned(t *testing.T) {
	// Distinct text per review: reconcile falls back to matching on text, and
	// real reviews never share wording (all 127 in data/reviews.json are unique).
	existing := []Review{rev("a", 5, longText), rev("gone", 5, "quite different wording "+longText)}
	fetched := []Review{rev("a", 5, longText)}
	out, tomb, skipped := reconcile(existing, fetched, "2026-07-31", 5)
	if skipped {
		t.Fatal("unexpected skip")
	}
	if len(tomb) != 1 || tomb[0].ID != "gone" {
		t.Fatalf("expected 'gone' tombstoned, got %+v", tomb)
	}
	if out[1].Removed != "2026-07-31" {
		t.Fatalf("removal date not set, got %q", out[1].Removed)
	}
	if out[0].Removed != "" {
		t.Fatal("present review wrongly tombstoned")
	}
}

func TestDemotedToFourStarsIsTombstoned(t *testing.T) {
	existing := []Review{rev("a", 5, longText)}
	fetched := []Review{rev("a", 4, longText)} // author edited the rating down
	_, tomb, _ := reconcile(existing, fetched, "2026-07-31", 5)
	if len(tomb) != 1 {
		t.Fatalf("expected demoted review tombstoned, got %d", len(tomb))
	}
}

func TestShortenedBelowMinWordsIsTombstoned(t *testing.T) {
	existing := []Review{rev("a", 5, longText)}
	fetched := []Review{rev("a", 5, shortText)}
	_, tomb, _ := reconcile(existing, fetched, "2026-07-31", 5)
	if len(tomb) != 1 {
		t.Fatalf("expected shortened review tombstoned, got %d", len(tomb))
	}
}

func TestStillQualifyingReviewIsNotRewritten(t *testing.T) {
	// Content refresh is deliberately out of scope: a still-qualifying review
	// must be left byte-for-byte alone even if its text changed upstream.
	existing := []Review{rev("a", 5, longText)}
	fetched := []Review{rev("a", 5, longText+" plus an edit")}
	out, tomb, _ := reconcile(existing, fetched, "2026-07-31", 5)
	if len(tomb) != 0 {
		t.Fatal("qualifying review should not be tombstoned")
	}
	if out[0].Text != longText {
		t.Fatal("text was rewritten; reconcile must not refresh content")
	}
}

func TestAlreadyTombstonedIsLeftAlone(t *testing.T) {
	prior := rev("a", 5, longText)
	prior.Removed = "2020-01-01"
	out, tomb, _ := reconcile([]Review{prior}, nil, "2026-07-31", 5)
	if len(tomb) != 0 {
		t.Fatal("already-tombstoned review reprocessed")
	}
	if out[0].Removed != "2020-01-01" {
		t.Fatalf("original removal date overwritten: %q", out[0].Removed)
	}
}

func TestLegacyIDStillMatchesByText(t *testing.T) {
	// Records captured while the site used the Places API carry places/... ids.
	// The Business Profile API never returns those, so matching on ID alone
	// would tombstone them even though the review is still live. The text must
	// rescue them. This is not hypothetical: a dry run proposed removing all six
	// such records at once.
	stored := rev("places/ChIJvQmm/reviews/Ci9DQUlR", 5, longText)
	live := rev("accounts/123/locations/456/reviews/AbFvOq", 5, longText)
	out, tomb, _ := reconcile([]Review{stored}, []Review{live}, "2026-07-31", 5)
	if len(tomb) != 0 {
		t.Fatalf("legacy-id record tombstoned despite its text being live: %+v", tomb)
	}
	if out[0].Removed != "" {
		t.Fatal("legacy-id record marked removed")
	}
}

func TestTextMatchStillHonoursTheDisplayBar(t *testing.T) {
	// Matching by text must not smuggle in a review that no longer qualifies.
	stored := rev("places/old", 5, longText)
	live := rev("accounts/new", 4, longText) // same text, downgraded to 4 stars
	_, tomb, _ := reconcile([]Review{stored}, []Review{live}, "2026-07-31", 5)
	if len(tomb) != 1 {
		t.Fatalf("text-matched but demoted review should be tombstoned, got %d", len(tomb))
	}
}

func TestGenuinelyGoneIsStillTombstonedWithTextFallback(t *testing.T) {
	// The text fallback must not make removal impossible.
	stored := rev("accounts/123/reviews/gone", 5, longText)
	other := rev("accounts/123/reviews/other", 5, "completely different wording here "+longText)
	_, tomb, _ := reconcile([]Review{stored}, []Review{other}, "2026-07-31", 5)
	if len(tomb) != 1 {
		t.Fatalf("a genuinely absent review should still be tombstoned, got %d", len(tomb))
	}
}

func TestEmptyIDIsNeverTombstoned(t *testing.T) {
	// A review with no ID cannot be proven absent, so it must never be removed.
	out, tomb, _ := reconcile([]Review{{ID: "", Text: ""}}, nil, "2026-07-31", 5)
	if len(tomb) != 0 || out[0].Removed != "" {
		t.Fatal("review without an ID was tombstoned")
	}
}

func TestExceedingMaxRemovalsChangesNothing(t *testing.T) {
	var existing []Review
	for _, id := range []string{"a", "b", "c", "d"} {
		existing = append(existing, rev(id, 5, longText))
	}
	out, tomb, skipped := reconcile(existing, nil, "2026-07-31", 3)
	if !skipped {
		t.Fatal("expected skip when over the cap")
	}
	if len(tomb) != 4 {
		t.Fatalf("expected all 4 reported as would-remove, got %d", len(tomb))
	}
	for _, r := range out {
		if r.Removed != "" {
			t.Fatalf("review %s mutated despite skip", r.ID)
		}
	}
	// and the caller's own slice must be untouched
	for _, r := range existing {
		if r.Removed != "" {
			t.Fatalf("input slice mutated for %s", r.ID)
		}
	}
}

func TestExactlyAtMaxRemovalsIsApplied(t *testing.T) {
	var existing []Review
	for _, id := range []string{"a", "b", "c"} {
		existing = append(existing, rev(id, 5, longText))
	}
	out, tomb, skipped := reconcile(existing, nil, "2026-07-31", 3)
	if skipped {
		t.Fatal("should not skip at exactly the cap")
	}
	if len(tomb) != 3 {
		t.Fatalf("expected 3 tombstoned, got %d", len(tomb))
	}
	for _, r := range out {
		if r.Removed != "2026-07-31" {
			t.Fatalf("%s not tombstoned", r.ID)
		}
	}
}

func TestQualifies(t *testing.T) {
	cases := []struct {
		name string
		r    Review
		want bool
	}{
		{"five star long", rev("a", 5, longText), true},
		{"four star long", rev("a", 4, longText), false},
		{"five star short", rev("a", 5, shortText), false},
		{"zero rating", rev("a", 0, longText), false},
	}
	for _, c := range cases {
		if got := qualifies(c.r); got != c.want {
			t.Errorf("%s: qualifies = %v, want %v", c.name, got, c.want)
		}
	}
}
