package paxos

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// The message log records the messages that this paxos agent has recieved so
// that it can recover its state in case of failure.
type MsgLog struct {
	mtx   *sync.Mutex
	Log   []Msg
	Fpath string
	fd    *os.File // the file that back the message log
}

// Create a new message log of given size, that corresponds to file at path. It
// is associated with an agent in case it needs to recover it from a previously
// saved state.
func NewMsgLog(sz int, path string, a *Agent, try_recover bool) (*MsgLog, error) {
	l := &MsgLog{}
	l.mtx = &sync.Mutex{}
	l.Log = make([]Msg, sz)
	l.Fpath = path
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0777)
	}
	if _, err := os.Stat(path); os.IsExist(err) {
		// file exists therefore recover from it
		if !try_recover {
			// if we aren't supposed to use it to recover
			// assume that we should delete it
			// this is good for testing or running clean instances of Paxos
			err = os.Remove(path)
			if err != nil {
				log.Print("Error Deleting Log File")
			}
		} else {
			log.Print("File Exists")
			f, err := os.Open(path)
			if err != nil {
				log.Print("Error Opening File Exists")
				return nil, err
			}
			err = l.Recover(a, f)
			if err != nil {
				return nil, err
			}
			err = f.Close()
			if err != nil {
				return nil, err
			}
		}
	}
	log.Print("File Does not Exist: ", path)
	var err error
	l.fd, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Print("Failed to Open File")
		return nil, err
	}
	return l, nil
}

// Recover recovers the agent from the saved MsgLog.
func (l *MsgLog) Recover(a *Agent, f *os.File) error {
	dec := json.NewDecoder(f)
	for {
		var m Msg
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		log.Print("Recovered: ", m)
		// append the recovered message to the in memory log
		l.Log = append(l.Log, m)
		if m.Type == ClientRequest {
			a.StartRequest(m.Round, m.Value, m.Request, false)
		} else {
			a.handlePaxosMessage(m, false)
		}
	}
	// after recovering never assume that I am the leader
	a.isLeader = false
	a.leader = nil
	return nil
}

// Resize resizes the in-memory message log
func (l *MsgLog) Resize(n int) {
	tl := make([]Msg, n*2)
	copy(tl, l.Log)
	l.Log = tl
}

// Flush flushes the message log to disk.
func (l *MsgLog) Flush() {
	err := l.fd.Sync()
	if err != nil {
		log.Fatal("Failed to Flush File: ", err)
	}
}

// Append appends a new message to the message log and flushes it to disk.
func (l *MsgLog) Append(m Msg) {
	l.mtx.Lock()
	l.Log = append(l.Log, m)
	// Append this one message to the file
	by, err := json.Marshal(m)
	if err != nil {
		log.Fatal("Error Appending To Log:", err)
	}
	_, err = l.fd.Write(by)
	if err != nil {
		log.Fatal("Error Appending To Log:", err)
	}
	l.Flush()
	l.mtx.Unlock()
}

// ValueEntry is an entry into the ValuesLog
type ValueEntry struct {
	Committed bool        // Indicates whether this entry is committed
	Val       Value       // The value that has been committed here
	Request   RequestInfo // The request associated with this value
}

// A ValueLog is a sequence of Values that the the Paxos node has accepted
// in the order that it has accepted it.
type ValueLog struct {
	cv *sync.Cond
	sync.Mutex
	Log []ValueEntry
}

// NewValueLog creates and initializes log.
func NewValueLog(sz int) *ValueLog {
	l := &ValueLog{}
	l.cv = sync.NewCond(l)
	l.Log = make([]ValueEntry, 0, sz)
	return l
}

// InsertAt inserts a value with given request info at a given index in the
// log. There is no need to commit this to disk because it can be recovered
// from the message log.
func (l *ValueLog) InsertAt(i int, v Value, r RequestInfo) {
	l.Lock()
	if i >= len(l.Log) {
		t := make([]ValueEntry, ((i+1)*3)/2)
		copy(t, l.Log)
		l.Log = t

	}
	l.Log[i] = ValueEntry{true, v, r}
	l.Unlock()
	l.cv.Broadcast() // Signal those that might be streaming the log that a new entry is availible.
}

// Append a value to the log.
func (l *ValueLog) Append(v Value, r RequestInfo) {
	l.Lock()
	l.Log = append(l.Log, ValueEntry{true, v, r})
	l.Unlock()
	l.cv.Broadcast()
}

// Stream returns a channel of values that corresponds with this log in order.
func (l *ValueLog) Stream() <-chan ValueEntry {
	ch := make(chan ValueEntry)
	go func() {
		i := 0
		for {
			// wait for this entry to be filled
			l.Lock()
			for i >= len(l.Log) || !l.Log[i].Committed {
				log.Println("Stream: Waiting for entry:", i)
				l.cv.Wait()
			}
			log.Println("Stream: Streaming Entry:", i)
			ch <- l.Log[i]
			i++
			l.Unlock()
		}
	}()
	return ch
}
