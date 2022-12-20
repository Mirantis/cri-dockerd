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

package utils

// AlphaRequestToV1Request converts cri.v1alpha2 requests to cri.v1 requests
func AlphaReqToV1Req(
	alphaRequest interface{ Marshal() ([]byte, error) },
	v1Request interface{ Unmarshal(_ []byte) error },
) error {
	p, err := alphaRequest.Marshal()
	if err != nil {
		return err
	}

	if err = v1Request.Unmarshal(p); err != nil {
		return err
	}
	return nil
}

// V1ResponseToAlphaResponse converts cri.v1 responses to cri.v1alpha2 responses
func V1ResponseToAlphaResponse(
	v1Response interface{ Marshal() ([]byte, error) },
	alphaResponse interface{ Unmarshal(_ []byte) error },
) error {
	p, err := v1Response.Marshal()
	if err != nil {
		return err
	}

	if err = alphaResponse.Unmarshal(p); err != nil {
		return err
	}
	return nil
}
