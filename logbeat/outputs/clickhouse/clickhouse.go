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
	var log = logp.NewLogger(loggerName)

	conf := defaultConfig
	if err := cfg.Unpack(&conf); err != nil {
		return outputs.Fail(err)
	}

	if len(conf.Table) == 0 {
		errMsg := "clickhouse: the table name must be set"
		log.Error(errMsg)
		return outputs.Fail(errors.New(errMsg))
	}

	if len(conf.Columns) == 0 {
		errMsg := "clickhouse: the table columns must be set"
		log.Error(errMsg)
		return outputs.Fail(errors.New(errMsg))
	}

	cli := newClient(conf, observer)
	return outputs.SuccessNet(conf.Queue, false, conf.BulkMaxSize, maxRetries, []outputs.NetworkClient{cli})
}
