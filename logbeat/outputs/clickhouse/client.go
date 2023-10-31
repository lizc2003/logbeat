package clickhouse

import (
	"context"
	"errors"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/backoff"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"github.com/elastic/elastic-agent-libs/logp"
	"strings"
	"sync"
)

type client struct {
	addr      []string
	username  string
	password  string
	table     string
	columns   []string
	insertSql string

	mutex    sync.Mutex
	log      *logp.Logger
	conn     driver.Conn
	observer outputs.Observer
	done     chan struct{}
	backoff  backoff.Backoff
}

func newClient(c clickhouseConfig, observer outputs.Observer) *client {
	done := make(chan struct{})

	return &client{
		log:       logp.NewLogger(loggerName),
		observer:  observer,
		done:      done,
		backoff:   backoff.NewEqualJitterBackoff(done, backoffInit, backoffMax),
		addr:      c.Addr,
		username:  c.Username,
		password:  c.Password,
		table:     c.Table,
		columns:   c.Columns,
		insertSql: "INSERT INTO " + c.Table + "(" + strings.Join(c.Columns, ",") + ")",
	}
}

func (c *client) Connect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.conn != nil {
		c.log.Info("connection reuse")
		return nil
	}

	conn, err := openConn(c.addr, c.username, c.password)
	if err != nil {
		c.log.Errorf("clickhouse open fail: %v", err)
		return err
	}

	c.log.Infof("clickhouse connect ok")
	c.conn = conn
	return nil
}

func (c *client) Close() error {
	c.log.Infof("clickhouse connection close")
	var err error
	if c.conn != nil {
		err = c.conn.Close()
		c.conn = nil
	}
	close(c.done)
	return err
}

func (c *client) String() string {
	return "clickhouse(" + strings.Join(c.addr, ",") + ")"
}

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

// publish events
func (c *client) publishEvents(ctx context.Context, data []publisher.Event) ([]publisher.Event, error) {
	szTotal := len(data)
	if szTotal == 0 {
		return nil, nil
	}

	okEvents, rows := extractDataFromEvent(c.log, data, c.columns)
	dropped := szTotal - len(okEvents)
	c.log.Debugf("[check data] total: %d, dropped: %d", szTotal, dropped)
	if dropped > 0 {
		c.observer.Dropped(dropped)
	}

	rowCount := len(rows)
	if rowCount == 0 {
		return nil, nil
	}

	batch, err := c.conn.PrepareBatch(ctx, c.insertSql)
	if err != nil {
		c.log.Errorf("batch prepare fail: %v", err)
		return okEvents, err
	}
	for i := 0; i < rowCount; i++ {
		err := batch.Append(rows[i]...)
		if err != nil {
			c.log.Errorf("batch append fail: %v", err)
			return okEvents, err
		}
	}

	err = batch.Send()
	if err != nil {
		c.log.Errorf("batch send fail: %v", err)
		return okEvents, err
	}

	c.observer.Acked(rowCount)
	return nil, nil
}

func openConn(addr []string, username, password string) (driver.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: addr,
		Auth: clickhouse.Auth{
			Database: "",
			Username: username,
			Password: password,
		},
		ClientInfo: clickhouse.ClientInfo{
			Products: []struct {
				Name    string
				Version string
			}{
				{Name: "logbeat-client", Version: "1.0"},
			},
		},

		//TLS: &tls.Config{
		//	InsecureSkipVerify: true,
		//},
	})
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err = conn.Ping(ctx); err != nil {
		return nil, err
	}

	return conn, nil
}

// extractDataFromEvent extract data
func extractDataFromEvent(
	log *logp.Logger,
	data []publisher.Event,
	columns []string,
) ([]publisher.Event, [][]any) {
	var okEvents []publisher.Event
	var to [][]any
	for _, event := range data {
		row, err := matchFields(event.Content, columns)
		if err != nil {
			log.Errorf("clickhouse match field error: %v", err)
			continue
		}
		to = append(to, row)
		// match successed then append ok-events
		okEvents = append(okEvents, event)
	}
	return okEvents, to
}

// matchFields match field format
func matchFields(content beat.Event, columns []string) ([]any, error) {
	row := make([]any, 0, len(columns))
	for _, col := range columns {
		if _, ok := content.Fields[col]; !ok {
			return nil, errors.New("format error")
		}
		val, err := content.GetValue(col)
		if err != nil {
			return nil, err
		}
		// strict mode
		//if val == nil {
		//	return nil, errors.New("row field is empty")
		//}
		row = append(row, val)
	}
	return row, nil
}
