package http

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
)

// sa referrer: https://manual.sensorsdata.cn/sa/3.0/zh_cn/tech_super_access-150668120.html

type saEncoder struct {
	buf *bytes.Buffer
}

func newSaEncoder(buf *bytes.Buffer) *saEncoder {
	if buf == nil {
		buf = bytes.NewBuffer(nil)
	}
	return &saEncoder{buf}
}

func (b *saEncoder) Reset() {
	b.buf.Reset()
}

func (b *saEncoder) AddHeader(header *http.Header) {
	header.Add("Content-Type", "application/x-www-form-urlencoded")
	header.Add("Content-Encoding", "gzip")
}

func (b *saEncoder) Reader() io.Reader {
	return b.buf
}

func (b *saEncoder) Marshal(obj any) error {
	b.Reset()

	var msg []byte
	bArray := false

	switch v := obj.(type) {
	case eventRaw:
		msg = v[originMsgKey]
	case []eventRaw:
		sz := len(v)
		if sz == 0 {
			return errors.New("empty event list")
		} else if sz == 1 {
			msg = v[0][originMsgKey]
		} else {
			bArray = true
			var buf bytes.Buffer
			buf.WriteByte('[')
			for i := 0; i < sz; i++ {
				if i > 0 {
					buf.WriteByte(',')
				}
				buf.Write(v[i][originMsgKey])
			}
			buf.WriteByte(']')
			msg = buf.Bytes()
		}
	default:
		return errors.New("unknown obj type")
	}

	if len(msg) == 0 {
		return errors.New("empty event")
	}

	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		return err
	}
	gz.Write(msg)
	gz.Close()

	if bArray {
		b.buf.WriteString("data_list=")
	} else {
		b.buf.WriteString("data=")
	}
	b.buf.WriteString(url.QueryEscape(base64.StdEncoding.EncodeToString(buf.Bytes())))
	b.buf.WriteString("&gzip=1")
	return nil
}

func (b *saEncoder) AddRaw(raw any) error {
	return errors.New("not implemented")
}

func (b *saEncoder) Add(meta, obj any) error {
	return errors.New("not implemented")
}
