package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasPrefix(t *testing.T) {
	assert.True(t, HasPrefix("10.0.0.0/16", "10."))
	assert.True(t, HasPrefix("10.0.0.0/16", "10.0.0.0/16"))
	assert.False(t, HasPrefix("10.0.0.0/16", "/16"))
	assert.False(t, HasPrefix("10.0.0.0/16", "192."))
	assert.True(t, HasPrefix("10.0.0.0/16", ""))
}
