package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"discord-agent/internal/llm"
	_ "modernc.org/sqlite"
)

type Fact struct {
	Key   string
	Value string
}

type ErrorEntry struct {
	ID      int64
	Time    time.Time
	Message string
}

type Store struct {
	db       *sql.DB
	mu       sync.Mutex
	maxFacts int
}

func Open(path string, maxFacts int) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // tiny VPS: keep SQLite single-writer and low memory
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS conversation (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scope TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			ts INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_conv_scope ON conversation(scope, id)`,
		`CREATE TABLE IF NOT EXISTS facts (
			scope TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (scope, key)
		)`,
		`CREATE TABLE IF NOT EXISTS summaries (
			scope TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS stats (
			key TEXT PRIMARY KEY,
			value INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS errors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts INTEGER NOT NULL,
			message TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("建立資料庫失敗: %w", err)
		}
	}
	if maxFacts < 1 {
		maxFacts = 50
	}
	return &Store{db: db, maxFacts: maxFacts}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) SetMaxFacts(n int) {
	if n > 0 {
		s.maxFacts = n
	}
}

func (s *Store) AddExchange(scope, userText, replyText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	for _, m := range []struct{ role, content string }{{"user", userText}, {"assistant", replyText}} {
		if _, err := tx.Exec(`INSERT INTO conversation(scope,role,content,ts) VALUES(?,?,?,?)`, scope, m.role, m.content, now); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM conversation WHERE scope=? AND id NOT IN
		(SELECT id FROM conversation WHERE scope=? ORDER BY id DESC LIMIT 200)`, scope, scope); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Recent(scope string, limit int) ([]llm.Message, error) {
	if limit < 1 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT role,content FROM conversation WHERE scope=?
		ORDER BY id DESC LIMIT ?`, scope, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []llm.Message
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err != nil {
			return nil, err
		}
		msgs = append(msgs, llm.Message{Role: role, Content: content})
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, rows.Err()
}

func (s *Store) Clear(scope string) error {
	_, err := s.db.Exec(`DELETE FROM conversation WHERE scope=?`, scope)
	return err
}

func (s *Store) MessageCount(scope string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM conversation WHERE scope=?`, scope).Scan(&n)
	return n, err
}

func (s *Store) Remember(scope, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO facts(scope,key,value,updated_at) VALUES(?,?,?,?)
		ON CONFLICT(scope,key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		scope, key, value, time.Now().Unix()); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM facts WHERE scope=? AND key NOT IN
		(SELECT key FROM facts WHERE scope=? ORDER BY updated_at DESC LIMIT ?)`, scope, scope, s.maxFacts); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Facts(scope string, limit int) ([]Fact, error) {
	if limit < 1 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT key,value FROM facts WHERE scope=? ORDER BY updated_at DESC LIMIT ?`, scope, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var facts []Fact
	for rows.Next() {
		var f Fact
		if err := rows.Scan(&f.Key, &f.Value); err != nil {
			return nil, err
		}
		facts = append(facts, f)
	}
	return facts, rows.Err()
}

func (s *Store) SetSummary(scope, content string) error {
	_, err := s.db.Exec(`INSERT INTO summaries(scope,content,updated_at) VALUES(?,?,?)
		ON CONFLICT(scope) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at`,
		scope, content, time.Now().Unix())
	return err
}

func (s *Store) Summary(scope string) (string, error) {
	var content string
	err := s.db.QueryRow(`SELECT content FROM summaries WHERE scope=?`, scope).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
}

func (s *Store) IncrStat(key string) error {
	_, err := s.db.Exec(`INSERT INTO stats(key,value) VALUES(?,1)
		ON CONFLICT(key) DO UPDATE SET value=value+1`, key)
	return err
}

// AddError keeps at most 50 sanitized error messages for admin review.
func (s *Store) AddError(message string) error {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO errors(ts,message) VALUES(?,?)`, time.Now().Unix(), message); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM errors WHERE id NOT IN
		(SELECT id FROM errors ORDER BY id DESC LIMIT 50)`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Errors(limit int) ([]ErrorEntry, error) {
	if limit < 1 || limit > 50 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id,ts,message FROM errors ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ErrorEntry
	for rows.Next() {
		var e ErrorEntry
		var ts int64
		if err := rows.Scan(&e.ID, &ts, &e.Message); err != nil {
			return nil, err
		}
		e.Time = time.Unix(ts, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) Stats() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT key,value FROM stats`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var v int
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
