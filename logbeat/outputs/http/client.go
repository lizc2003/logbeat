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
	"context"
	"encoding/json"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/transport/httpcommon"
	"net/http"
	"strings"
	"unsafe"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/backoff"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/publisher"
)

type clientSettings struct {
	Username         string
	Password         string
	Headers          map[string]string
	CompressionLevel int
	URL              string
	BatchMode        bool
	Channel          string
	AppId            string

	Transport httpcommon.HTTPTransportSettings
	Observer  outputs.Observer
}

type client struct {
	url       string
	batchMode bool
	channel   string
	appId     string

	conn     *Connection
	observer outputs.Observer
	done     chan struct{}
	backoff  backoff.Backoff
}

type eventRaw map[string]json.RawMessage

func newClient(s clientSettings) (*client, error) {
	conn, err := NewConnection(&s)
	if err != nil {
		return nil, err
	}

	done := make(chan struct{})

	cli := &client{
		conn:      conn,
		observer:  s.Observer,
		done:      done,
		backoff:   backoff.NewEqualJitterBackoff(done, backoffInit, backoffMax),
		url:       s.URL,
		batchMode: s.BatchMode,
		channel:   s.Channel,
		appId:     s.AppId,
	}

	return cli, nil
}

// Connect establishes a connection to the clients sink.
func (c *client) Connect() error {
	c.conn.log.Info("Connect")
	return c.conn.Connect()
}

// Close closes a connection.
func (c *client) Close() error {
	c.conn.log.Info("Close")
	err := c.conn.Close()
	close(c.done)
	return err
}

func (c *client) String() string {
	return "httpout(" + c.url + ")"
}

// Publish sends events to the clients sink.
func (c *client) Publish(ctx context.Context, batch publisher.Batch) error {
	events := batch.Events()
	c.observer.NewBatch(len(events))
	rest, err := c.publishEvents(ctx, events)
	if len(rest) != 0 {
		c.observer.Failed(len(rest))
		batch.RetryEvents(rest)
	} else {
		batch.ACK()
	}

	backoff.WaitOnError(c.backoff, err)
	return err
}

// publishEvents posts all events to the http endpoint. On error a slice with all
// events not published will be returned.
func (c *client) publishEvents(ctx context.Context, data []publisher.Event) ([]publisher.Event, error) {
	szTotal := len(data)
	if szTotal == 0 {
		return nil, nil
	}

	okEvents, evts := extractDataFromEvent(c.conn.log, data, c.channel, c.appId)
	dropped := szTotal - len(okEvents)
	c.conn.log.Debugf("[check data] total: %d, dropped: %d", szTotal, dropped)
	if dropped > 0 {
		c.observer.Dropped(dropped)
	}

	count := len(evts)
	if count == 0 {
		return nil, nil
	}

	if c.batchMode {
		err := c.doPublish(evts)
		if err != nil {
			return okEvents, err
		}
	} else {
		for i, event := range evts {
			err := c.doPublish(event)
			if err != nil {
				if i > 0 {
					c.observer.Acked(i)
				}
				return okEvents[i:], err
			}
		}
	}

	c.observer.Acked(count)
	return nil, nil
}

func (c *client) doPublish(body any) error {
	_, _, err := c.conn.RequestURL(http.MethodPost, c.url, body)
	if err != nil {
		if err == ErrJSONEncodeFailed {
			// don't retry unencodable values
			return nil
		}
		return err
	}

	//if c.channel == channelShushu {
	//	var result struct {
	//		Code int `json:"code"`
	//	}
	//	err = json.Unmarshal(resp, &result)
	//	if err != nil {
	//		result.Code = 1
	//	}
	//	if result.Code != 0 {
	//		c.conn.log.Errorf("shushu publish fail, code: %d", result.Code)
	//	}
	//}

	return nil
}

func extractDataFromEvent(
	log *logp.Logger,
	data []publisher.Event,
	channel, appId string,
) ([]publisher.Event, []eventRaw) {
	var okEvents []publisher.Event
	var to []eventRaw
	for _, event := range data {
		e, err := makeEvent(event.Content, channel, appId)
		if err != nil {
			log.Errorf("httpout make event error: %v", err)
			continue
		}

		to = append(to, e)
		okEvents = append(okEvents, event)
	}
	return okEvents, to
}

func makeEvent(v beat.Event, channel string, appId string) (eventRaw, error) {
	msgBody, err := getMessageBody(v)
	if err != nil {
		return nil, err
	}

	var ret eventRaw

	switch channel {
	case channelShushu: // {"appid":"xxx","data":{}}
		// doc: https://docs.thinkingdata.cn/ta-manual/latest/installation/installation_menu/restful_api.html#_2-2-%E6%95%B0%E6%8D%AE%E6%8E%A5%E6%94%B6%E6%8E%A5%E5%8F%A3-%E6%8F%90%E4%BA%A4%E6%96%B9%E5%BC%8F%E4%B8%BA-raw
		ret = make(eventRaw)
		ret["data"] = json.RawMessage(msgBody)
		ret["appid"] = json.RawMessage(strings.Join([]string{"\"", appId, "\""}, ""))
	case channelOpenObserve: // {"content":"xxx"}
		ret = make(eventRaw)
		b, _ := json.Marshal(msgBody)
		ret["content"] = b
	default:
		if err = json.Unmarshal(UnsafeStr2Bytes(msgBody), &ret); err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func getMessageBody(v beat.Event) (string, error) {
	msgVal, err := v.Fields.GetValue("message")
	if err != nil {
		return "", err
	}

	if msg, ok := msgVal.(string); ok {
		if len(msg) != 0 {
			return msg, nil
		}
	}
	return "", ErrEmptyMessage
}

func UnsafeStr2Bytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
