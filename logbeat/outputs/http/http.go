package http

import (
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
)

func init() {
	outputs.RegisterType("http", makeHttp)
}

func makeHttp(
	im outputs.IndexManager,
	beat beat.Info,
	observer outputs.Observer,
	cfg *config.C,
) (outputs.Group, error) {
	log := logp.NewLogger(loggerName)
	log.Infof("make httpout")

	conf := defaultConfig
	if err := cfg.Unpack(&conf); err != nil {
		return outputs.Fail(err)
	}

	hosts, err := outputs.ReadHostList(cfg)
	if err != nil {
		return outputs.Fail(err)
	}

	if proxyURL := conf.Transport.Proxy.URL; proxyURL != nil && !conf.Transport.Proxy.Disable {
		log.Debugf("breaking down proxy URL. Scheme: '%s', host[:port]: '%s', path: '%s'", proxyURL.Scheme, proxyURL.Host, proxyURL.Path)
		log.Infof("Using proxy URL: %s", proxyURL)
	}

	clients := make([]outputs.NetworkClient, len(hosts))
	for i, host := range hosts {
		hostURL, err := common.MakeURL(conf.Protocol, conf.Path, host, 0)
		if err != nil {
			log.Errorf("Invalid host param set: %s, Error: %+v", host, err)
			return outputs.Fail(err)
		}

		var cli outputs.NetworkClient
		cli, err = newClient(clientSettings{
			URL:              hostURL,
			Observer:         observer,
			Username:         conf.Username,
			Password:         conf.Password,
			CompressionLevel: conf.CompressionLevel,
			BatchMode:        conf.BatchMode,
			Headers:          conf.Headers,
			Channel:          conf.Channel,
			AppId:            conf.AppId,
		})

		if err != nil {
			return outputs.Fail(err)
		}

		clients[i] = cli
	}
	return outputs.SuccessNet(conf.LoadBalance, conf.BulkMaxSize, maxRetries, clients)
}
