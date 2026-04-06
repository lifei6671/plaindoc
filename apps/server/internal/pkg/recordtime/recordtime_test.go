package recordtime

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{
			name:     "empty returns zero",
			input:    "",
			expected: time.Time{},
		},
		{
			name:     "rfc3339 nano",
			input:    "2026-04-06T12:34:56.123456789Z",
			expected: time.Date(2026, 4, 6, 12, 34, 56, 123456789, time.UTC),
		},
		{
			name:     "space separated with offset",
			input:    "2026-04-06 12:34:56+00:00",
			expected: time.Date(2026, 4, 6, 12, 34, 56, 0, time.UTC),
		},
		{
			name:     "space separated without zone",
			input:    "2026-04-06 12:34:56",
			expected: time.Date(2026, 4, 6, 12, 34, 56, 0, time.UTC),
		},
		{
			name:     "unknown format returns zero",
			input:    "not-a-time",
			expected: time.Time{},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := Parse(tc.input)
			if !got.Equal(tc.expected) {
				if got.IsZero() != tc.expected.IsZero() {
					t.Fatalf("Parse(%q) zero mismatch: got=%v expected=%v", tc.input, got, tc.expected)
				}
				if !got.IsZero() {
					t.Fatalf("Parse(%q)=%s, expected %s", tc.input, got.Format(time.RFC3339Nano), tc.expected.Format(time.RFC3339Nano))
				}
			}
		})
	}
}

func TestParseNullable(t *testing.T) {
	t.Parallel()

	raw := "2026-04-06 12:34:56"
	got := ParseNullable(&raw)
	if got == nil {
		t.Fatal("expected non-nil parsed time")
	}

	expected := time.Date(2026, 4, 6, 12, 34, 56, 0, time.UTC)
	if !got.Equal(expected) {
		t.Fatalf("ParseNullable(%q)=%s, expected %s", raw, got.Format(time.RFC3339Nano), expected.Format(time.RFC3339Nano))
	}

	if ParseNullable(nil) != nil {
		t.Fatal("expected nil when input is nil")
	}

	blank := "   "
	if ParseNullable(&blank) != nil {
		t.Fatal("expected nil when input is blank")
	}
}
