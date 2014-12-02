package paxos

import (
	"errors"
	"time"
)

// startTimeout is the initial timeout to sleep for after a failed connection
// attempt. Clients connect using an exponential backoff algorithm.
var startTimeout time.Duration = LeaderTimeout / 2

// multTimeout is the multiple for the sleep bewteen connection attempts.
var multTimeout time.Duration = 2

// maxTimeout is the max sleep time between connection attempts.
var maxTimeout time.Duration = 2 * time.Minute

// RedirectError is returned if a Client is redirected
var RedirectError error = errors.New("Redirect")
