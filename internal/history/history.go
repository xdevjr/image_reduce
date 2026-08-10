// Package history mantém o histórico de conversões em memória e em disco (JSONL).
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// MaxEntries limita o número de eventos mantidos em memória.
const MaxEntries = 500

// Status possíveis de um evento.
const (
	StatusQueued     = "queued"
	StatusConverting = "converting"
	StatusDone       = "done"
	StatusSkipped    = "skipped"
	StatusError      = "error"
)

// Event representa uma entrada do histórico de conversão.
type Event struct {
	ID        string    `json:"id"`
	File      string    `json:"file"`
	Source    string    `json:"source"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	Output    string    `json:"output,omitempty"`
	SizeIn    int64     `json:"size_in"`
	SizeOut   int64     `json:"size_out"`
	Duration  string    `json:"duration,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Store é um armazenamento thread-safe de eventos com persistência JSONL.
type Store struct {
	mu      sync.RWMutex
	entries []*Event
	path    string
}

// New cria um Store e carrega eventos previamente persistidos.
func New(path string) *Store {
	s := &Store{path: path}
	s.load()
	return s
}

func (s *Store) load() {
	f, err := os.Open(s.path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		s.entries = append(s.entries, &e)
	}
	s.trim()
}

// Add registra um novo evento e o persiste.
func (s *Store) Add(e *Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	s.trim()
	s.appendToFile(e)
}

func (s *Store) trim() {
	if len(s.entries) > MaxEntries {
		s.entries = s.entries[len(s.entries)-MaxEntries:]
	}
}

func (s *Store) appendToFile(e *Event) {
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	f.Write(append(data, '\n'))
}

// List retorna uma cópia dos eventos na ordem de inserção.
func (s *Store) List() []*Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Event, len(s.entries))
	copy(out, s.entries)
	return out
}

// Clear apaga todos os eventos e o arquivo persistido.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	os.Remove(s.path)
}