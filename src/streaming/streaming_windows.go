// +build windows

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

package streaming

import (
	"bytes"
	"fmt"
	"io"
)

func (r *StreamingRuntime) portForward(
	podSandboxID string,
	port int32,
	stream io.ReadWriteCloser,
) error {
	stderr := new(bytes.Buffer)
	if err != nil {
		return fmt.Errorf("%v: %s", err, stderr.String())
	}

	return nil
}
