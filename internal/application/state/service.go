package state

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"time"

	statecontract "agent-harness/internal/contract/state"
	statedomain "agent-harness/internal/domain/state"
	"agent-harness/internal/domain/statepath"
	stateport "agent-harness/internal/port/state"
)

const stateBucket = "state"

type Dependencies struct {
	StateDir        func() string
	StatePath       func(dir, key string) string
	OpenStore       func(dir string) (stateport.Store, error)
	ExistingRecords stateport.ExistingReader
	Now             func() time.Time
}

type Service struct {
	dependencies Dependencies
}

func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies}
}

func (service *Service) Write(key, content string) (statecontract.StateResult, error) {
	key, err := statepath.NormalizeKey(key)
	if err != nil {
		return statecontract.StateResult{OK: false, StateDir: service.stateDir()}, err
	}
	dir := service.stateDir()
	store, err := service.dependencies.OpenStore(dir)
	if err != nil {
		return statecontract.StateResult{OK: false, StateDir: dir}, err
	}
	var result statecontract.StateResult
	err = store.WithSpan(context.Background(), func(context.Context) error {
		record := statecontract.RecordEnvelope{
			SchemaVersion: statecontract.SchemaVersion,
			Key:           key,
			Content:       content,
			UpdatedAt:     service.now().UTC().Format(time.RFC3339Nano),
			Bytes:         len([]byte(content)),
		}
		path, writeErr := service.writeRecord(store, dir, key, record)
		result = statecontract.StateResult{OK: writeErr == nil, StateDir: dir, Path: path, Record: record}
		return writeErr
	})
	if result.OK || result.StateDir != "" {
		return result, err
	}
	return statecontract.StateResult{OK: false, StateDir: dir}, err
}

func (service *Service) Read(key string) (statecontract.StateResult, error) {
	key, err := statepath.NormalizeKey(key)
	if err != nil {
		return statecontract.StateResult{OK: false, StateDir: service.stateDir()}, err
	}
	return service.read(service.stateDir(), key)
}

func (service *Service) read(dir, key string) (statecontract.StateResult, error) {
	path := service.dependencies.StatePath(dir, key)
	raw, ok, err := service.dependencies.ExistingRecords.GetExisting(dir, stateBucket, key)
	if err != nil {
		return statecontract.StateResult{OK: false, StateDir: dir, Path: path}, err
	}
	if !ok {
		return statecontract.StateResult{OK: false, StateDir: dir, Path: path}, &fs.PathError{Op: "read", Path: path, Err: fs.ErrNotExist}
	}
	record, err := DecodeRecord(key, raw)
	if err != nil {
		return statecontract.StateResult{OK: false, StateDir: dir, Path: path}, err
	}
	return statecontract.StateResult{OK: true, StateDir: dir, Path: path, Record: record}, nil
}

func DecodeRecord(key string, data []byte) (statecontract.RecordEnvelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record statecontract.RecordEnvelope
	if err := decoder.Decode(&record); err != nil {
		return statecontract.RecordEnvelope{}, statecontract.Invalid("")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return statecontract.RecordEnvelope{}, statecontract.Invalid("")
	}
	if err := statedomain.ValidateRecord(key, record); err != nil {
		return statecontract.RecordEnvelope{}, statecontract.Invalid("")
	}
	return record, nil
}

func (service *Service) List() (statecontract.StateListResult, error) {
	dir := service.stateDir()
	store, err := service.dependencies.OpenStore(dir)
	if err != nil {
		return statecontract.StateListResult{OK: false, StateDir: dir}, err
	}
	keys, err := store.List(stateBucket)
	if err != nil {
		return statecontract.StateListResult{OK: false, StateDir: dir}, err
	}
	records := []statecontract.StateListEntry{}
	for _, key := range keys {
		if _, err := statepath.NormalizeKey(key); err != nil {
			continue
		}
		result, err := service.read(dir, key)
		if err != nil {
			continue
		}
		records = append(records, statecontract.StateListEntry{
			Key:           result.Record.Key,
			UpdatedAt:     result.Record.UpdatedAt,
			Bytes:         result.Record.Bytes,
			SchemaVersion: result.Record.SchemaVersion,
		})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Key < records[j].Key })
	listKeys := make([]string, 0, len(records))
	for _, record := range records {
		listKeys = append(listKeys, record.Key)
	}
	return statecontract.StateListResult{OK: true, StateDir: dir, Keys: listKeys, Records: records}, nil
}

func (service *Service) WriteRecord(dir, key string, record statecontract.RecordEnvelope) (string, error) {
	key, err := statepath.NormalizeKey(key)
	if err != nil {
		return "", err
	}
	if record.Key != "" && record.Key != key {
		return "", fmt.Errorf("state record key %q does not match write key %q", record.Key, key)
	}
	store, err := service.dependencies.OpenStore(dir)
	if err != nil {
		return "", err
	}
	var path string
	err = store.WithSpan(context.Background(), func(context.Context) error {
		var writeErr error
		path, writeErr = service.writeRecord(store, dir, key, record)
		return writeErr
	})
	return path, err
}

func (service *Service) writeRecord(store stateport.Store, dir, key string, record statecontract.RecordEnvelope) (string, error) {
	path := service.dependencies.StatePath(dir, key)
	if err := statedomain.ValidateRecord(key, record); err != nil {
		return path, err
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return path, err
	}
	if err := store.Mutate([]stateport.Mutation{{Bucket: stateBucket, ID: key, Data: append(raw, '\n')}}); err != nil {
		return path, err
	}
	return path, nil
}

func (service *Service) Delete(key string) error {
	key, err := statepath.NormalizeKey(key)
	if err != nil {
		return err
	}
	store, err := service.dependencies.OpenStore(service.stateDir())
	if err != nil {
		return err
	}
	return store.Mutate([]stateport.Mutation{{Bucket: stateBucket, ID: key, Delete: true}})
}

func (service *Service) Update(key string, transform func(statecontract.RecordEnvelope) (statecontract.RecordEnvelope, error)) (statecontract.StateResult, error) {
	key, err := statepath.NormalizeKey(key)
	if err != nil {
		return statecontract.StateResult{OK: false, StateDir: service.stateDir()}, err
	}
	dir := service.stateDir()
	store, err := service.dependencies.OpenStore(dir)
	if err != nil {
		return statecontract.StateResult{OK: false, StateDir: dir}, err
	}
	var result statecontract.StateResult
	err = store.WithSpan(context.Background(), func(context.Context) error {
		current, readErr := service.read(dir, key)
		if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
			return readErr
		}
		next, transformErr := transform(current.Record)
		if transformErr != nil {
			return transformErr
		}
		if next.Key == "" && next.SchemaVersion == 0 && next.Content == "" {
			result = statecontract.StateResult{OK: true, StateDir: dir}
			return nil
		}
		path, writeErr := service.writeRecord(store, dir, key, next)
		result = statecontract.StateResult{OK: writeErr == nil, StateDir: dir, Path: path, Record: next}
		return writeErr
	})
	if result.OK || result.StateDir != "" {
		return result, err
	}
	return statecontract.StateResult{OK: false, StateDir: dir}, err
}

func (service *Service) WithKeyLock(ctx context.Context, dir, key string, fn func(context.Context) error) error {
	if _, err := statepath.NormalizeKey(key); err != nil {
		return err
	}
	store, err := service.dependencies.OpenStore(dir)
	if err != nil {
		return err
	}
	return store.WithSpan(ctx, fn)
}

func (service *Service) stateDir() string {
	return service.dependencies.StateDir()
}

func (service *Service) now() time.Time {
	if service.dependencies.Now != nil {
		return service.dependencies.Now()
	}
	return time.Now()
}
