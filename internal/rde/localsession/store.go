// Package localsession persists `rde claude` session records locally so they
// can be resumed later (`rde claude --resume` / `--continue`).
//
// Records are grouped by the local repository they were started from, mirroring
// how Claude Code organizes its own transcripts by project:
//
//	<config-dir>/rde/projects/<encoded-repo-path>/<rde-session-id>.json
//
// where <config-dir> is the bitrise config directory (see internal/config.Dir).
// One file per `rde claude` invocation. The record is written immediately at
// session creation (so an abrupt stop still leaves something resumable) and
// enriched over the session's life by the metadata monitor with the
// AI-generated title and description.
package localsession

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bitrise-io/bitrise-cli/internal/config"
)

// Record is one resumable `rde claude` session. JSON tags define the on-disk
// shape; this file is read only by the CLI, so the contract is internal.
type Record struct {
	RDESessionID    string    `json:"rde_session_id"`
	WorkspaceID     string    `json:"workspace_id"`
	Name            string    `json:"name"`              // initial generated name (claude-<hex>)
	ClaudeSessionID string    `json:"claude_session_id"` // UUID we pass to `claude --session-id`
	AITitle         string    `json:"ai_title,omitempty"`
	Description     string    `json:"description,omitempty"`
	Repo            string    `json:"repo,omitempty"`      // origin remote URL
	RepoPath        string    `json:"repo_path,omitempty"` // local repo root; the project key
	Branch          string    `json:"branch,omitempty"`
	RemoteRepoDir   string    `json:"remote_repo_dir,omitempty"` // dir Claude runs in on the session
	LastStatus      string    `json:"last_status,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DisplayName is the human label for a record: the AI-generated title once it
// exists, otherwise the initial generated name.
func (r Record) DisplayName() string {
	if r.AITitle != "" {
		return r.AITitle
	}
	return r.Name
}

// projectsDir is the root holding every project's records.
func projectsDir() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rde", "projects"), nil
}

// projectDir returns the directory holding records for the given local repo
// path. The key encodes the path into a single filesystem-safe segment.
func projectDir(repoPath string) (string, error) {
	root, err := projectsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, projectKey(repoPath)), nil
}

// projectKey encodes a local repo path into one filesystem-safe segment by
// replacing every character outside [A-Za-z0-9._-] with '-'. The exact scheme
// doesn't need to match Claude Code's — the store only reads its own keys — it
// just has to be stable for a given path.
func projectKey(repoPath string) string {
	var b strings.Builder
	b.Grow(len(repoPath))
	for _, r := range repoPath {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	key := b.String()
	if key == "" {
		key = "-"
	}
	return key
}

// Save writes (or overwrites) the record for its repo + RDE session ID. It
// stamps UpdatedAt (and CreatedAt if unset) and writes atomically so a reader
// never sees a half-written file. Directories are 0700, the file 0600 — the
// record carries no secrets, but it sits alongside auth.yaml/config.yaml and
// follows the same locked-in perms.
func Save(rec Record) error {
	if rec.RDESessionID == "" {
		return errors.New("record has no RDE session ID")
	}
	if rec.RepoPath == "" {
		return errors.New("record has no repo path")
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now

	dir, err := projectDir(rec.RepoPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create session store dir: %w", err)
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session record: %w", err)
	}
	data = append(data, '\n')

	final := filepath.Join(dir, rec.RDESessionID+".json")
	tmp, err := os.CreateTemp(dir, rec.RDESessionID+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp session record: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // best-effort cleanup if rename already moved it
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close() //nolint:errcheck,gosec // returning the chmod error; close failure is secondary
		return fmt.Errorf("chmod temp session record: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close() //nolint:errcheck,gosec // returning the write error; close failure is secondary
		return fmt.Errorf("write session record: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close session record: %w", err)
	}
	if err := os.Rename(tmpName, final); err != nil {
		return fmt.Errorf("save session record: %w", err)
	}
	return nil
}

// Load returns the record for the given repo + RDE session ID. A missing
// record returns os.ErrNotExist.
func Load(repoPath, rdeSessionID string) (Record, error) {
	dir, err := projectDir(repoPath)
	if err != nil {
		return Record{}, err
	}
	return readRecord(filepath.Join(dir, rdeSessionID+".json"))
}

// ListByProject returns every record for the given local repo, newest-updated
// first. A missing project directory is not an error — it returns an empty
// slice. Unparseable files are skipped so one corrupt record can't hide the
// rest.
func ListByProject(repoPath string) ([]Record, error) {
	dir, err := projectDir(repoPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read session store: %w", err)
	}
	var recs []Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		rec, err := readRecord(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // skip corrupt/partial records
		}
		recs = append(recs, rec)
	}
	sort.SliceStable(recs, func(i, j int) bool {
		return recs[i].UpdatedAt.After(recs[j].UpdatedAt)
	})
	return recs, nil
}

// Latest returns the most-recently-updated record for the repo, or ok=false
// when none exist.
func Latest(repoPath string) (Record, bool, error) {
	recs, err := ListByProject(repoPath)
	if err != nil {
		return Record{}, false, err
	}
	if len(recs) == 0 {
		return Record{}, false, nil
	}
	return recs[0], true, nil
}

// Remove deletes the record for the given repo + RDE session ID. A missing
// record is not an error.
func Remove(repoPath, rdeSessionID string) error {
	dir, err := projectDir(repoPath)
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(dir, rdeSessionID+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func readRecord(path string) (Record, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is derived from the config dir + our own session IDs, not user input
	if err != nil {
		return Record{}, err
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return Record{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return rec, nil
}
