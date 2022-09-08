package featureflag

import (
	"time"

	"gopkg.in/launchdarkly/go-sdk-common.v2/lduser"
	ld "gopkg.in/launchdarkly/go-server-sdk.v5"
)

// Client is the interface to the feature flag system
type Client interface {
	BoolVariation(key string, userKey string, defaultVal bool) (bool, error)
	Close() error
}

// LaunchDarkly is a LaunchDarkly-backed feature flag client
type LaunchDarkly struct {
	ldc *ld.LDClient
}

// NewLaunchDarkly instantiates a LaunchDarkly-backed feature flag client
func NewLaunchDarkly(apiKey string, waitFor time.Duration) (Client, error) {
	ldClient, err := ld.MakeClient(apiKey, waitFor)
	if err != nil {
		return nil, err
	}
	return &LaunchDarkly{ldc: ldClient}, nil
}

// BoolVariation checks whether a feature flag is on or off for a user
func (l *LaunchDarkly) BoolVariation(key string, userKey string, defaultVal bool) (bool, error) {
	user := lduser.NewUser(userKey)
	return l.ldc.BoolVariation(key, user, defaultVal)
}

// Close shuts down the LaunchDarkly-backed feature flag client
func (l *LaunchDarkly) Close() error {
	return l.ldc.Close()
}

// DefaultValueAlways is a fallback feature flag client that always returns the default value,
// meant to be used when an external system like LaunchDarkly is down
type DefaultValueAlways struct{}

// BoolVariation is a dummy function that always returns the default value
func (d DefaultValueAlways) BoolVariation(key string, userKey string, defaultVal bool) (bool, error) {
	return defaultVal, nil
}

// Close is a dummy function
func (d DefaultValueAlways) Close() error {
	return nil
}
