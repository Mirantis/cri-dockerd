/*
Copyright 2021 Mirantis

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"fmt"
	"sync"
)

// ErrCorruptCheckpoint error is reported when checksum does not match
var ErrCorruptCheckpoint = fmt.Errorf("checkpoint is corrupted")

// ErrCheckpointNotFound is reported when checkpoint is not found for a given key
var ErrCheckpointNotFound = fmt.Errorf("checkpoint is not found")

// Checkpoint provides the process checkpoint data
type Checkpoint interface {
	MarshalCheckpoint() ([]byte, error)
	UnmarshalCheckpoint(blob []byte) error
	VerifyChecksum() error
}

// CheckpointManager provides the interface to manage checkpoint
type CheckpointManager interface {
	// CreateCheckpoint persists checkpoint in CheckpointStore. checkpointKey is the key for utilstore to locate checkpoint.
	// For file backed utilstore, checkpointKey is the file name to write the checkpoint data.
	CreateCheckpoint(checkpointKey string, checkpoint Checkpoint) error
	// GetCheckpoint retrieves checkpoint from CheckpointStore.
	GetCheckpoint(checkpointKey string, checkpoint Checkpoint) error
	// WARNING: RemoveCheckpoint will not return error if checkpoint does not exist.
	RemoveCheckpoint(checkpointKey string) error
	// ListCheckpoint returns the list of existing checkpoints.
	ListCheckpoints() ([]string, error)
}

// checkPointImpl is an implementation of CheckpointManager. It persists checkpoints in CheckpointStore
type checkPointImpl struct {
	path  string
	store Store
	mutex sync.Mutex
}

// NewCheckpointManager returns a new instance of a checkpoint manager
func NewCheckpointManager(checkpointDir string) (CheckpointManager, error) {
	fstore, err := NewFileStore(checkpointDir, DefaultFs{})
	if err != nil {
		return nil, err
	}

	return &checkPointImpl{path: checkpointDir, store: fstore}, nil
}

// CreateCheckpoint persists checkpoint in CheckpointStore.
func (manager *checkPointImpl) CreateCheckpoint(checkpointKey string, checkpoint Checkpoint) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	blob, err := checkpoint.MarshalCheckpoint()
	if err != nil {
		return err
	}
	return manager.store.Write(checkpointKey, blob)
}

// GetCheckpoint retrieves checkpoint from CheckpointStore.
func (manager *checkPointImpl) GetCheckpoint(checkpointKey string, checkpoint Checkpoint) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	blob, err := manager.store.Read(checkpointKey)
	if err != nil {
		if err == ErrKeyNotFound {
			return ErrCheckpointNotFound
		}
		return err
	}
	err = checkpoint.UnmarshalCheckpoint(blob)
	if err == nil {
		err = checkpoint.VerifyChecksum()
	}
	return err
}

// RemoveCheckpoint will not return error if checkpoint does not exist.
func (manager *checkPointImpl) RemoveCheckpoint(checkpointKey string) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	return manager.store.Delete(checkpointKey)
}

// ListCheckpoints returns the list of existing checkpoints.
func (manager *checkPointImpl) ListCheckpoints() ([]string, error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	keys, err := manager.store.List()
	if err != nil {
		return []string{}, fmt.Errorf("failed to list checkpoint store: %v", err)
	}
	return keys, nil
}
