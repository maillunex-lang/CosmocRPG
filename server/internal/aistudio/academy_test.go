package aistudio

import (
	"strings"
	"testing"
)

func TestSuiteOn300(t *testing.T) {
	c := gen300(t)
	res := RunSuite(&c, nil, 40)
	for _, r := range res {
		t.Logf("%-12s %4d/%-4d %s", r.Kind, r.Passed, r.Total, strings.Join(first(r.Failures, 3), " | "))
	}
	for _, r := range res {
		if r.Passed != r.Total {
			t.Errorf("%s: провалено %d из %d: %v", r.Kind, r.Total-r.Passed, r.Total, first(r.Failures, 6))
		}
	}
}

func first(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
