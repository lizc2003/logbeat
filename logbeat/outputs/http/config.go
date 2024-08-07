// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package http

import (
	"errors"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/transport/httpcommon"
	"time"
)

const (
	loggerName = "httpout"

	idleConnectTimeout = 1 * time.Minute
	maxRetries         = 3
	backoffInit        = 1 * time.Second
	backoffMax         = 60 * time.Second

	channelShushu      = "shushu"
	channelOpenObserve = "openobserve"
	channelSa          = "sa"
)

var (
	ErrNotConnected = errors.New("not connected")         // failure due to client having no valid connection
	ErrEncodeFailed = errors.New("message encode failed") // encoding failures
	ErrEmptyMessage = errors.New("empty message")
)

type httpConfig struct {
	Protocol         string            `config:"protocol"`
	Path             string            `config:"path"`
	Headers          map[string]string `config:"headers"`
	Username         string            `config:"username"`
	Password         string            `config:"password"`
	LoadBalance      bool              `config:"loadbalance"`
	CompressionLevel int               `config:"compression_level" validate:"min=0, max=9"`
	BulkMaxSize      int               `config:"bulk_max_size"`
	BatchMode        bool              `config:"batch_mode"`
	Channel          string            `config:"channel"`
	AppId            string            `config:"app_id"`
	Queue            config.Namespace  `config:"queue"`

	Transport httpcommon.HTTPTransportSettings `config:",inline"`
}

func (c *httpConfig) Validate() error {
	return nil
}

var (
	defaultConfig = httpConfig{
		BulkMaxSize: 50,
		LoadBalance: true,
		Transport:   httpcommon.DefaultHTTPTransportSettings(),
	}
)
