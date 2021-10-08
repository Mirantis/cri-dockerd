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
	"errors"
	"hash"
	"hash/fnv"

	"github.com/davecgh/go-spew/spew"
)

// Checksum is the data to be stored as checkpoint
type Checksum uint64

// Verify verifies that passed checksum is same as calculated checksum
func (cs Checksum) Verify(data interface{}) error {
	if cs != NewChecksum(data) {
		return errors.New("checkpoint is corrupted")
	}
	return nil
}

// NewChecksum returns the Checksum of checkpoint data
func NewChecksum(data interface{}) Checksum {
	return Checksum(getChecksum(data))
}

// Get returns calculated checksum of checkpoint data
func getChecksum(data interface{}) uint64 {
	hashData := fnv.New32a()
	deepHashObject(hashData, data)
	return uint64(hashData.Sum32())
}

// deepHashObject writes specified object to hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func deepHashObject(hashed hash.Hash, objectToWrite interface{}) {
	hashed.Reset()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	printer.Fprintf(hashed, "%#v", objectToWrite)
}
