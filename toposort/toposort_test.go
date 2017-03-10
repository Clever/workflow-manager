package toposort

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildDegreeMap(t *testing.T) {
	in := map[string][]string{
		"a": {"b", "c"},
		"b": {"c"},
		"c": {},
	}

	exp := map[string]int{
		"c": 2,
		"b": 1,
		"a": 0,
	}
	expNoInc := []string{"a"}

	out, noInc := buildDegreeMap(in)
	assert.Equal(t, exp, out)
	assert.Equal(t, expNoInc, noInc)
}

type sortTestCase struct {
	title       string
	in          map[string][]string
	expected    [][]string
	shouldError bool
}

var sortTestCases = []sortTestCase{
	{
		title: "Simple dependencies",
		in: map[string][]string{
			"a": {"b", "c"},
			"b": {"c"},
			"c": {},
		},
		expected:    [][]string{{"c"}, {"b"}, {"a"}},
		shouldError: false,
	},
	{
		title: "Small cycle",
		in: map[string][]string{
			"a": {"b"},
			"b": {"a"},
		},
		shouldError: true,
	},
	{
		title: "No dependencies",
		in: map[string][]string{
			"a": {},
		},
		expected:    [][]string{{"a"}},
		shouldError: false,
	},
	{
		title: "Longer cycle",
		in: map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"a"},
		},
		shouldError: true,
	},
	{
		title:       "No inputs",
		in:          map[string][]string{},
		expected:    [][]string{},
		shouldError: false,
	},
	{
		title: "parallel deps",
		in: map[string][]string{
			"e": {"d", "c"},
			"d": {"a", "b"},
			"a": {},
			"c": {},
			"b": {},
		},
		expected:    [][]string{{"a", "b"}, {"d", "c"}, {"e"}},
		shouldError: false,
	},
	{
		title: "complex parallel deps",
		in: map[string][]string{
			"e": {"d", "c", "f"},
			"d": {"a", "b"},
			"a": {},
			"c": {},
			"b": {"c"},
			"f": {"g"},
			"g": {"h"},
			"h": {},
		},
		expected:    [][]string{{"c", "h"}, {"a", "b", "g"}, {"d", "f"}, {"e"}},
		shouldError: false,
	},
}

func TestSort(t *testing.T) {
	for _, test := range sortTestCases {
		t.Logf("Testing %s... ", test.title)
		actual, err := Sort(test.in)
		if test.shouldError {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.expected, actual)
		}
	}
}
