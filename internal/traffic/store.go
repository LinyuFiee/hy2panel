package traffic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxStateFileSize = 16 << 20

var (
	// ErrStateCorrupt means the state file cannot be safely decoded or violates
	// format invariants. Save refuses to replace such an existing file.
	ErrStateCorrupt = errors.New("traffic state is corrupt")
	// ErrUnknownSchema means the state belongs to a newer or unsupported schema.
	// Save refuses to replace such an existing file.
	ErrUnknownSchema = errors.New("unknown traffic state schema")
)

// Store persists traffic checkpoints at a path chosen by the caller.
type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Load returns a validated, bounded state. A missing file produces an empty
// current-schema state so first startup requires no special case.
func (s *Store) Load() (PersistentState, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return PersistentState{}, errors.New("traffic state path is empty")
	}
	return loadStateFile(s.path)
}

// Save validates first, then writes a 0600 temporary file in the destination
// directory, syncs it, renames it, and syncs the parent directory. An existing
// corrupt or unknown-schema file is deliberately left untouched.
func (s *Store) Save(state PersistentState) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return errors.New("traffic state path is empty")
	}
	state.SchemaVersion = StateSchemaVersion
	state.SavedAt = state.SavedAt.UTC()
	if err := validatePersistentState(&state); err != nil {
		return err
	}
	if _, err := os.Stat(s.path); err == nil {
		if _, loadErr := loadStateFile(s.path); loadErr != nil {
			if errors.Is(loadErr, ErrStateCorrupt) || errors.Is(loadErr, ErrUnknownSchema) {
				return fmt.Errorf("refusing to overwrite unreadable traffic state: %w", loadErr)
			}
			return loadErr
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if len(data)+1 > maxStateFileSize {
		return errors.New("traffic state exceeds size limit")
	}
	data = append(data, '\n')
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(directory, ".traffic-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := replaceFile(tempPath, s.path); err != nil {
		return err
	}
	return syncDirectory(directory)
}

func loadStateFile(path string) (PersistentState, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return newPersistentState(), nil
	}
	if err != nil {
		return PersistentState{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return PersistentState{}, err
	}
	if info.Size() > maxStateFileSize {
		return PersistentState{}, fmt.Errorf("%w: file exceeds size limit", ErrStateCorrupt)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxStateFileSize+1))
	if err != nil {
		return PersistentState{}, err
	}
	if len(data) > maxStateFileSize {
		return PersistentState{}, fmt.Errorf("%w: file exceeds size limit", ErrStateCorrupt)
	}
	var envelope struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return PersistentState{}, fmt.Errorf("%w: invalid JSON", ErrStateCorrupt)
	}
	if envelope.SchemaVersion != StateSchemaVersion {
		return PersistentState{}, fmt.Errorf("%w: %d", ErrUnknownSchema, envelope.SchemaVersion)
	}
	var state PersistentState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return PersistentState{}, fmt.Errorf("%w: %v", ErrStateCorrupt, err)
	}
	if err := requireStateEOF(decoder); err != nil {
		return PersistentState{}, err
	}
	if err := validatePersistentState(&state); err != nil {
		return PersistentState{}, err
	}
	if state.Open == nil {
		state.Open = make(map[string]OpenFlow)
	}
	return state, nil
}

func requireStateEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing JSON data", ErrStateCorrupt)
	}
	return nil
}

func validatePersistentState(state *PersistentState) error {
	if state == nil {
		return errors.New("traffic state is nil")
	}
	if state.SchemaVersion != StateSchemaVersion {
		return fmt.Errorf("%w: %d", ErrUnknownSchema, state.SchemaVersion)
	}
	if !validSourceIdentity(state.Source) {
		return fmt.Errorf("%w: invalid source identity", ErrStateCorrupt)
	}
	if len(state.History) > MaxHistoryPoints {
		return fmt.Errorf("%w: too many history points", ErrStateCorrupt)
	}
	if len(state.Open) > MaxOpenFlows {
		return fmt.Errorf("%w: too many open flows", ErrStateCorrupt)
	}
	previous := time.Time{}
	for index := range state.History {
		point := &state.History[index]
		if point.Minute.IsZero() || !point.Minute.Equal(point.Minute.UTC().Truncate(time.Minute)) {
			return fmt.Errorf("%w: invalid history minute", ErrStateCorrupt)
		}
		if !previous.IsZero() && !point.Minute.After(previous) {
			return fmt.Errorf("%w: history is not strictly ordered", ErrStateCorrupt)
		}
		if point.Total != point.Traffic.Total() {
			return fmt.Errorf("%w: history total mismatch", ErrStateCorrupt)
		}
		previous = point.Minute
	}
	for id, flow := range state.Open {
		if id == "" || len(id) > 256 || !flow.Protocol.valid() || flow.Start.IsZero() || !flow.Start.Equal(flow.Start.UTC()) {
			return fmt.Errorf("%w: invalid open flow", ErrStateCorrupt)
		}
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil && !isIgnorableDirectorySyncError(err) {
		return err
	}
	return nil
}
