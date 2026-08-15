package monitors

import "testing"

func TestDiffFlagsVerdictFlipsAndConfidenceMoves(t *testing.T) {
	base := Check{Verdict: "supported", Confidence: 0.85}

	cases := []struct {
		name    string
		hasPrev bool
		next    Check
		want    bool
	}{
		{"first check never changed", false, Check{Verdict: "refuted", Confidence: 0.1}, false},
		{"identical", true, Check{Verdict: "supported", Confidence: 0.85}, false},
		{"small confidence drift", true, Check{Verdict: "supported", Confidence: 0.80}, false},
		{"verdict flip", true, Check{Verdict: "unresolved", Confidence: 0.85}, true},
		{"confidence drop at threshold", true, Check{Verdict: "supported", Confidence: 0.75}, true},
		{"confidence rise past threshold", true, Check{Verdict: "supported", Confidence: 0.99}, true},
	}
	for _, tc := range cases {
		changed, note := diff(base, tc.hasPrev, tc.next)
		if changed != tc.want {
			t.Errorf("%s: changed=%v want %v", tc.name, changed, tc.want)
		}
		if changed && note == "" {
			t.Errorf("%s: changed but note empty", tc.name)
		}
		if !changed && note != "" {
			t.Errorf("%s: unchanged but note %q", tc.name, note)
		}
	}
}
