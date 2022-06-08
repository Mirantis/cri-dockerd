/*
Copyright 2022 Mirantis

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

package config

import (
	"fmt"
)

// HairpinMode is the type of network hairpin modes
type HairpinMode string

// HairpinModeValue implements pflag's Value interface
type HairpinModeValue struct {
	value string
}

// HairpinModeVar contains the value of the hairpin-mode flag
var HairpinModeVar HairpinModeValue

const (
	PromiscuousBridge HairpinMode = "promiscuous-bridge"
	HairpinVeth       HairpinMode = "hairpin-veth"
	HairpinNone       HairpinMode = "none"
)

var validHairpinModes = []HairpinMode{PromiscuousBridge, HairpinVeth, HairpinNone}

func (h *HairpinModeValue) String() string {
	if h.value == "" {
		return string(HairpinNone)
	}
	return h.value
}

func (h *HairpinModeValue) Set(mode string) error {
	for _, v := range validHairpinModes {
		if string(v) != mode {
			continue
		}
		h.value = mode
		return nil
	}
	return fmt.Errorf("%q is not a valid hairpin-mode value, must be one of: %+q", mode, validHairpinModes)
}

func (h *HairpinModeValue) Type() string {
	return "HairpinMode"
}

func (h *HairpinModeValue) Mode() HairpinMode {
	return HairpinMode(h.String())
}
