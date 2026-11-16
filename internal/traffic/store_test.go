package traffic

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func validState() PersistentState {
	minute := baseTestTime.Truncate(time.Minute)
	traffic := ProtocolCounters{
		Reality:      ByteCounters{Upload: 10, Download: 20},
		Hysteria2:    ByteCounters{Upload: 30, Download: 40},
		Unattributed: ByteCounters{Upload: 2, Download: 3},
	}
	return PersistentState{
		SchemaVersion: StateSchemaVersion,
		SavedAt:       baseTestTime,
		Initialized:   true,
		Source:        SourceIdentity{Instance: "instance-1", Epoch: "epoch-1"},
		Global:        ByteCounters{Upload: 1000, Download: 2000},
		Traffic:       traffic,
		Ahead:         ProtocolCounters{Reality: ByteCounters{Upload: 1}},
		Open: map[string]OpenFlow{
			"flow-1": {Protocol: ProtocolReality, Upload: 7, Download: 9, Start: baseTestTime.Add(-time.Minute)},
		},
		History: []MinutePoint{{Minute: minute, Traffic: traffic, Total: traffic.Total()}},
	}
}

func TestStoreSaveLoadAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "var", "lib", "hy2panel", "traffic.json")
	store := NewStore(path)
	state := validState()
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Traffic != state.Traffic || loaded.Global != state.Global || len(loaded.Open) != 1 || len(loaded.History) != 1 {
		t.Fatalf("loaded state differs: %#v", loaded)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".traffic-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temps) != 0 {
		t.Fatalf("temporary files remain: %v", temps)
	}
}

func TestStoreEmptyPathRejected(t *testing.T) {
	for _, path := range []string{"", "   "} {
		store := NewStore(path)
		if _, err := store.Load(); err == nil {
			t.Fatalf("Load accepted empty path %q", path)
		}
		if err := store.Save(validState()); err == nil {
			t.Fatalf("Save accepted empty path %q", path)
		}
	}
}

func TestStoreMissingFileReturnsEmptyState(t *testing.T) {
	state, err := NewStore(filepath.Join(t.TempDir(), "missing.json")).Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.SchemaVersion != StateSchemaVersion || state.Initialized || state.Open == nil {
		t.Fatalf("unexpected new state: %#v", state)
	}
}

func TestStoreRejectsCorruptAndUnknownWithoutOverwrite(t *testing.T) {
	tests := []struct {
		name      string
		contents  string
		wantError error
	}{
		{name: "corrupt", contents: `{broken`, wantError: ErrStateCorrupt},
		{name: "unknown schema", contents: `{"schema_version":999}`, wantError: ErrUnknownSchema},
		{name: "unknown field", contents: `{"schema_version":1,"mystery":true}`, wantError: ErrStateCorrupt},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "traffic.json")
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			store := NewStore(path)
			if _, err := store.Load(); err == nil || !errors.Is(err, test.wantError) {
				t.Fatalf("Load error = %v, want %v", err, test.wantError)
			}
			if err := store.Save(validState()); err == nil || !errors.Is(err, test.wantError) {
				t.Fatalf("Save error = %v, want protected existing file", err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != test.contents {
				t.Fatalf("protected state was overwritten: %q", data)
			}
		})
	}
}

func TestStoreLimits(t *testing.T) {
	t.Run("file size", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "traffic.json")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(maxStateFileSize + 1); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := NewStore(path).Load(); err == nil || !errors.Is(err, ErrStateCorrupt) {
			t.Fatalf("expected oversized state rejection, got %v", err)
		}
	})
	t.Run("history count", func(t *testing.T) {
		state := validState()
		state.History = make([]MinutePoint, MaxHistoryPoints+1)
		for index := range state.History {
			state.History[index].Minute = baseTestTime.Add(time.Duration(index) * time.Minute)
		}
		if err := NewStore(filepath.Join(t.TempDir(), "traffic.json")).Save(state); err == nil || !errors.Is(err, ErrStateCorrupt) {
			t.Fatalf("expected history limit error, got %v", err)
		}
	})
	t.Run("open count", func(t *testing.T) {
		state := validState()
		state.Open = make(map[string]OpenFlow, MaxOpenFlows+1)
		for index := 0; index <= MaxOpenFlows; index++ {
			state.Open[string(rune(index+1))] = OpenFlow{Protocol: ProtocolReality, Start: baseTestTime}
		}
		if err := NewStore(filepath.Join(t.TempDir(), "traffic.json")).Save(state); err == nil || !errors.Is(err, ErrStateCorrupt) {
			t.Fatalf("expected open flow limit error, got %v", err)
		}
	})
}

func TestStoreRejectsInvalidState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PersistentState)
	}{
		{name: "unordered history", mutate: func(state *PersistentState) { state.History = append(state.History, state.History[0]) }},
		{name: "history total mismatch", mutate: func(state *PersistentState) { state.History[0].Total.Upload++ }},
		{name: "invalid protocol", mutate: func(state *PersistentState) {
			flow := state.Open["flow-1"]
			flow.Protocol = "secret"
			state.Open["flow-1"] = flow
		}},
		{name: "zero flow start", mutate: func(state *PersistentState) {
			flow := state.Open["flow-1"]
			flow.Start = time.Time{}
			state.Open["flow-1"] = flow
		}},
		{name: "long flow id", mutate: func(state *PersistentState) {
			state.Open[strings.Repeat("x", 257)] = state.Open["flow-1"]
			delete(state.Open, "flow-1")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := validState()
			test.mutate(&state)
			if err := NewStore(filepath.Join(t.TempDir(), "traffic.json")).Save(state); err == nil {
				t.Fatal("expected invalid state to fail")
			}
		})
	}
}

func TestPersistentJSONContainsNoSensitiveFields(t *testing.T) {
	data, err := json.Marshal(validState())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{
		"source_ip", "sourceIP", "destination_ip", "destinationIP", "domain", "host", "rule", "secret", "token", "password",
	} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf("persistent JSON contains forbidden field %q: %s", forbidden, text)
		}
	}
	for _, required := range []string{"schema_version", "traffic", "open", "history", "instance", "epoch"} {
		if !strings.Contains(text, required) {
			t.Fatalf("persistent JSON missing expected field %q: %s", required, text)
		}
	}
}

func TestStoreReplacesValidState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.json")
	store := NewStore(path)
	first := validState()
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	second := validState()
	second.Traffic.Reality.Upload = 999
	second.History[0].Traffic = second.Traffic
	second.History[0].Total = second.Traffic.Total()
	if err := store.Save(second); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Traffic.Reality.Upload != 999 {
		t.Fatalf("valid state was not replaced: %#v", loaded.Traffic)
	}
}
