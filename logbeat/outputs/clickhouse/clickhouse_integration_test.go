package clickhouse

import (
	"context"
	"fmt"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/idxmgmt"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/outputs/outest"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/mapstr"
	"testing"
	"time"
)

func TestPublish(t *testing.T) {
	cfg := map[string]any{
		"addr":     []string{"10.45.11.35:9000"},
		"username": "default",
		"password": "123456",
		"table":    "ck_test",
		"columns":  []string{"id", "name", "created_at"},
	}

	err := prepare(cfg)
	if err != nil {
		t.Fatalf("Error preparing test env: %v", err)
		return
	}

	testPublishList(t, cfg)
}

func testPublishList(t *testing.T, cfg map[string]any) {
	batches := 2
	batchSize := 10

	output := newTestOutput(t, cfg)
	err := sendTestEvents(output, batches, batchSize)
	if err != nil {
		t.Fatalf("Error reading config: %v", err)
	}
}

func newTestOutput(t *testing.T, cfg map[string]any) outputs.Client {
	conf, err := config.NewConfigFrom(cfg)
	if err != nil {
		t.Fatalf("Error reading config: %v", err)
	}

	info := beat.Info{Beat: "libbeat"}
	// disable ILM if using specified index name
	im, _ := idxmgmt.DefaultSupport(nil, info, config.MustNewConfigFrom(map[string]any{"setup.ilm.enabled": "false"}))

	out, err := makeClickhouse(im, info, outputs.NewNilObserver(), conf)
	if err != nil {
		t.Fatalf("Failed to initialize clickhouse output: %v", err)
	}

	cli := out.Clients[0].(outputs.NetworkClient)
	if err := cli.Connect(); err != nil {
		t.Fatalf("Failed to connect to clickhouse host: %v", err)
	}

	return cli
}

func sendTestEvents(out outputs.Client, batches, N int) error {
	cnt := 1
	for b := 0; b < batches; b++ {
		events := make([]beat.Event, N)
		for n := range events {
			events[n] = createEvent(cnt)
			cnt++
		}

		batch := outest.NewBatch(events...)
		err := out.Publish(context.Background(), batch)
		if err != nil {
			return err
		}
	}

	return nil
}

func createEvent(id int) beat.Event {
	return beat.Event{
		Timestamp: time.Now(),
		Meta: mapstr.M{
			"ck-test": "ck-test-MetaValue",
		},
		Fields: mapstr.M{
			"id":         int64(id),
			"name":       fmt.Sprint("ck-test", id),
			"created_at": time.Now().UnixMilli(),
		},
	}
}

func prepare(cfg map[string]any) error {
	conn, err := openConn(cfg["addr"].([]string), cfg["username"].(string), cfg["password"].(string))
	if err != nil {
		return err
	}

	table := cfg["table"].(string)

	err = conn.Exec(context.Background(), fmt.Sprintf(`
		DROP TABLE IF EXISTS %s
	`, table))
	if err != nil {
		return err
	}

	err = conn.Exec(context.Background(), fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id         Int64,
			name       String,
			created_at DateTime(3)
		)
		ENGINE = MergeTree()
		PARTITION BY toYYYYMM(created_at)
		ORDER BY (created_at, id)
	`, table))
	return err
}
