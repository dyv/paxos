package paxos

import (
	"errors"
	"time"
)

var startTimeout time.Duration = LeaderTimeout / 2
var multTimeout time.Duration = 2
var endTimeout time.Duration = 2 * time.Minute
var RedirectError error = errors.New("Redirect")
