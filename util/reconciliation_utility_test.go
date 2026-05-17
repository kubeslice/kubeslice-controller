package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetOwnerLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "name shorter than 63 chars fits in one label",
			input: strings.Repeat("a", 50),
			expected: map[string]string{
				LabelResourceOwner: LabelValueResourceOwner,
				LabelManagedBy:     strings.Repeat("a", 50),
			},
		},
		{
			name:  "name exactly 63 chars fits in one label",
			input: strings.Repeat("a", 63),
			expected: map[string]string{
				LabelResourceOwner: LabelValueResourceOwner,
				LabelManagedBy:     strings.Repeat("a", 63),
			},
		},
		{
			name:  "name 64 chars splits across two labels",
			input: strings.Repeat("a", 63) + "b",
			expected: map[string]string{
				LabelResourceOwner:    LabelValueResourceOwner,
				LabelManagedBy:        strings.Repeat("a", 63),
				LabelManagedBy + "-1": "b",
			},
		},
		{
			// 126 = 2 × 63: post-loop would produce an empty-value label without the fix
			name:  "name exactly 126 chars produces exactly two name labels with no empty value",
			input: strings.Repeat("a", 63) + strings.Repeat("b", 63),
			expected: map[string]string{
				LabelResourceOwner:    LabelValueResourceOwner,
				LabelManagedBy:        strings.Repeat("a", 63),
				LabelManagedBy + "-1": strings.Repeat("b", 63),
			},
		},
		{
			name:  "name 127 chars splits across three labels",
			input: strings.Repeat("a", 63) + strings.Repeat("b", 63) + "c",
			expected: map[string]string{
				LabelResourceOwner:    LabelValueResourceOwner,
				LabelManagedBy:        strings.Repeat("a", 63),
				LabelManagedBy + "-1": strings.Repeat("b", 63),
				LabelManagedBy + "-2": "c",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GetOwnerLabel(tc.input)
			assert.Equal(t, tc.expected, result)
			for k, v := range result {
				assert.NotEmpty(t, v, "label key %q has empty value", k)
			}
		})
	}
}
