package http

import (
	"fmt"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/transport/httpcommon"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Connection struct {
	log     *logp.Logger
	encoder bodyEncoder
	http    *http.Client

	username string
	password string
	headers  map[string]string
}

func NewConnection(s *clientSettings) (*Connection, error) {
	log := logp.NewLogger(loggerName)

	u, err := url.Parse(s.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	if u.User != nil {
		s.Username = u.User.Username()
		s.Password, _ = u.User.Password()
		u.User = nil

		// Re-write URL without credentials.
		s.URL = u.String()
	}

	log.Infof("url: %s", s.URL)

	var encoder bodyEncoder
	compression := s.CompressionLevel
	if compression == 0 {
		encoder = newJSONEncoder(nil)
	} else {
		encoder, err = newGzipEncoder(compression, nil)
		if err != nil {
			return nil, err
		}
	}

	httpClient, err := s.Transport.Client(
		httpcommon.WithLogger(log),
		httpcommon.WithIOStats(s.Observer),
		httpcommon.WithKeepaliveSettings{IdleConnTimeout: idleConnectTimeout},
	)
	if err != nil {
		return nil, err
	}

	return &Connection{
		log:     log,
		http:    httpClient,
		encoder: encoder,

		username: s.Username,
		password: s.Password,
		headers:  s.Headers,
	}, nil
}

func (conn *Connection) Connect() error {
	return nil
}

// Close closes a connection.
func (conn *Connection) Close() error {
	conn.http.CloseIdleConnections()
	return nil
}

func (conn *Connection) Bulk(
	url string,
	body []interface{},
) (int, []byte, error) {
	if len(body) == 0 {
		return 0, nil, nil
	}

	if err := bulkEncode(conn.encoder, body); err != nil {
		conn.log.Warnf("Failed to json bulk body (%v)", err)
		return 0, nil, ErrJSONEncodeFailed
	}

	return conn.execRequest(http.MethodPost, url, conn.encoder.Reader())
}

func (conn *Connection) RequestURL(
	method, url string,
	body interface{},
) (int, []byte, error) {
	if body == nil {
		return conn.execRequest(method, url, nil)
	}

	if err := conn.encoder.Marshal(body); err != nil {
		conn.log.Warnf("Failed to json body (%v): %#v", err, body)
		return 0, nil, ErrJSONEncodeFailed
	}
	return conn.execRequest(method, url, conn.encoder.Reader())
}

func (conn *Connection) execRequest(
	method, url string,
	body io.Reader,
) (int, []byte, error) {
	req, err := http.NewRequest(method, url, body) //nolint:noctx // keep legacy behaviour
	if err != nil {
		conn.log.Warnf("Failed to create request %+v", err)
		return 0, nil, err
	}
	if body != nil {
		conn.encoder.AddHeader(&req.Header)
	}
	return conn.execHTTPRequest(req)
}

func (conn *Connection) execHTTPRequest(req *http.Request) (int, []byte, error) {
	req.Header.Add("Accept", "application/json")

	if conn.username != "" || conn.password != "" {
		req.SetBasicAuth(conn.username, conn.password)
	}

	for name, value := range conn.headers {
		if name == "Content-Type" || name == "Accept" {
			req.Header.Set(name, value)
		} else {
			req.Header.Add(name, value)
		}
	}

	// The stlib will override the value in the header based on the configured `Host`
	// on the request which default to the current machine.
	//
	// We use the normalized key header to retrieve the user configured value and assign it to the host.
	if host := req.Header.Get("Host"); host != "" {
		req.Host = host
	}

	resp, err := conn.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer closing(resp.Body, conn.log)

	status := resp.StatusCode
	obj, err := io.ReadAll(resp.Body)
	if err != nil {
		return status, nil, err
	}

	if status >= 300 {
		// add the response body with the error returned by Elasticsearch
		err = fmt.Errorf("%v: %s", resp.Status, obj)
	}

	return status, obj, err
}

func closing(c io.Closer, logger *logp.Logger) {
	err := c.Close()
	if err != nil {
		logger.Warn("Close failed with: %v", err)
	}
}

func addToURL(strUrl string, path string, params map[string]string) string {
	if strings.HasSuffix(strUrl, "/") && strings.HasPrefix(path, "/") {
		strUrl = strings.TrimSuffix(strUrl, "/")
	}
	if len(params) == 0 {
		return strUrl + path
	}

	values := url.Values{}
	for key, val := range params {
		values.Add(key, val)
	}

	return strings.Join([]string{
		strUrl, path, "?", values.Encode(),
	}, "")
}
