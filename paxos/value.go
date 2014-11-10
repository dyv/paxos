package paxos

type Equaler interface {
	Equal(Equaler) bool
}

type Value Equaler
