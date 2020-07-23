//
// Author:: Salim Afiune Maya (<afiune@lacework.net>)
// Copyright:: Copyright 2020, Lacework Inc.
// License:: Apache License, Version 2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package lwconfig

import (
	"path"

	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
)

// Config is the representation of the Lacework configuraiton file
// located at $HOME/.lacework.toml
//
// Example:
//
// updates = true
//
// [profiles]
//
//   [profiles.prod]
//   account = "prod"
//   api_key = "PROD_1234abcd"
//   api_secret = "_abcd1234"
//
//   [profiles.dev]
//   account = "dev"
//   api_key = "DEV_1234abcd"
//   api_secret = "_abcd1234"
//
//   [profiles.default]
//   account = "test"
//   api_key = "TEST_1234abcd"
//   api_secret = "_abcd1234"
//
type Config struct {
	Updates  bool                      `toml:"updates" json:"updates"`
	Profiles map[string]ProfileDetails `toml:"profiles" json:"profiles"`
}

type ProfileDetails struct {
	Account   string `toml:"account" json:"account"`
	ApiKey    string `toml:"api_key" json:"api_key" survey:"api_key"`
	ApiSecret string `toml:"api_secret" json:"api_secret" survey:"api_secret"`
}

func (c *ProfileDetails) Verify() error {
	if c.Account == "" {
		return errors.New("account missing")
	}
	if c.ApiKey == "" {
		return errors.New("api_key missing")
	}
	if c.ApiSecret == "" {
		return errors.New("api_secret missing")
	}
	return nil
}

func Default() Config {
	return Config{Updates: true}
}

func Load() (Config, error) {
	home, err := homedir.Dir()
	if err != nil {
		return Default(), err
	}

	// the Lacework config file is located at $HOME/.lacework.toml
	return LoadFromFile(path.Join(home, ".lacework.toml"))
}

func LoadFromFile(configPath string) (Config, error) {
	var config = Default()

	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		return config, errors.Wrap(err, "unable to decode config")
	}
	return config, nil
}
