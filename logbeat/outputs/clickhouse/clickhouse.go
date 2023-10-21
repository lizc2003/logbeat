package clickhouse

import (
	"errors"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
)

func init() {
	outputs.RegisterType("clickhouse", makeClickhouse)
}

func makeClickhouse(
	im outputs.IndexManager,
	beat beat.Info,
	observer outputs.Observer,
	cfg *config.C,
) (outputs.Group, error) {
	var log = logp.NewLogger("clickhouse")

	config := defaultConfig
	if err := cfg.Unpack(&config); err != nil {
		return outputs.Fail(err)
	}

	if len(config.Table) == 0 {
		errMsg := "clickhouse: the table name must be set"
		log.Error(errMsg)
		return outputs.Fail(errors.New(errMsg))
	}

	if len(config.Columns) == 0 {
		errMsg := "clickhouse: the table columns must be set"
		log.Error(errMsg)
		return outputs.Fail(errors.New(errMsg))
	}

	cli := newClient(config, observer)
	return outputs.SuccessNet(false, config.BulkMaxSize, config.MaxRetries, []outputs.NetworkClient{cli})
}
