package parsers

import (
	"bufio"
	"bytes"
	"io"
	"net/url"

	"github.com/tkuchiki/alp/errors"
	"github.com/tkuchiki/alp/helpers"
)

type Parser interface {
	Parse() (*ParsedHTTPStat, error)
	ReadBytes() int
	SetReadBytes(n int)
	Seek(n int) error
}

type ParsedHTTPStat struct {
	Uri          string
	Method       string
	Time         string
	ResponseTime float64
	BodyBytes    float64
	Status       int
	TraceID      string
	Entries      LogEntries
}

type LogEntries map[string]string

type statKeys struct {
	uri          string
	method       string
	time         string
	responseTime string
	requestTime  string
	bodyBytes    string
	status       string
	traceID      string
}

type statKey func(*statKeys)

func uriKey(s string) statKey {
	return func(sk *statKeys) {
		if s != "" {
			sk.uri = s
		}
	}
}

func methodKey(s string) statKey {
	return func(sk *statKeys) {
		if s != "" {
			sk.method = s
		}
	}
}

func timeKey(s string) statKey {
	return func(sk *statKeys) {
		if s != "" {
			sk.time = s
		}
	}
}

func responseTimeKey(s string) statKey {
	return func(sk *statKeys) {
		if s != "" {
			sk.responseTime = s
		}
	}
}

func requestTimeKey(s string) statKey {
	return func(sk *statKeys) {
		if s != "" {
			sk.requestTime = s
		}
	}
}

func bodyBytesKey(s string) statKey {
	return func(sk *statKeys) {
		if s != "" {
			sk.bodyBytes = s
		}
	}
}

func statusKey(s string) statKey {
	return func(sk *statKeys) {
		if s != "" {
			sk.status = s
		}
	}
}

func traceIDKey(s string) statKey {
	return func(sk *statKeys) {
		if s != "" {
			sk.traceID = s
		}
	}
}

func newStatKeys(sk ...statKey) *statKeys {
	sks := &statKeys{
		uri:          "uri",
		method:       "method",
		time:         "time",
		responseTime: "response_time",
		requestTime:  "request_time",
		bodyBytes:    "body_bytes",
		status:       "status",
		traceID:      "trace_id",
	}

	for _, s := range sk {
		s(sks)
	}

	return sks
}

func readline(reader *bufio.Reader) ([]byte, int, error) {
	var b []byte
	var i int
	var err error
	for {
		line, _err := reader.ReadBytes('\n')
		if _err == io.EOF && len(line) == 0 {
			err = io.EOF
			break
		}

		if _err != io.EOF && _err != nil {
			return []byte{}, 0, err
		}
		trimedLine := bytes.TrimRight(line, "\r\n")
		if len(trimedLine) > 0 {
			b = append(b, trimedLine...)
		} else {
			err = errors.SkipReadLineErr
		}

		size := len(line)
		i += size

		if line[size-1] == byte('\n') {
			break
		}
	}

	return b, i, err
}

func NewParsedHTTPStat(uri, method, time string, resTime, bodyBytes float64, status int, traceID string) *ParsedHTTPStat {
	return &ParsedHTTPStat{
		Uri:          uri,
		Method:       method,
		Time:         time,
		ResponseTime: resTime,
		BodyBytes:    bodyBytes,
		Status:       status,
		TraceID:      traceID,
	}
}

func toStats(parsedValue map[string]string, keys *statKeys, strictMode, queryString, qsIgnoreValues bool) (*ParsedHTTPStat, error) {
	u, err := url.Parse(parsedValue[keys.uri])
	if err != nil {
		return nil, errSkipReadLine(strictMode, err)
	}

	uri := normalizeURL(u, queryString, qsIgnoreValues)
	if uri == "" {
		return nil, errSkipReadLine(strictMode, err)
	}

	resTime, err := helpers.StringToFloat64(parsedValue[keys.responseTime])
	if err != nil {
		resTime, err = helpers.StringToFloat64(parsedValue[keys.requestTime])
		if err != nil {
			return nil, errSkipReadLine(strictMode, err)
		}
	}

	bodyBytes, err := helpers.StringToFloat64(parsedValue[keys.bodyBytes])
	if err != nil {
		return nil, errSkipReadLine(strictMode, err)
	}

	status, err := helpers.StringToInt(parsedValue[keys.status])
	if err != nil {
		return nil, errSkipReadLine(strictMode, err)
	}

	traceID := parsedValue[keys.traceID]
	if traceID == "" {
		return nil, errSkipReadLine(strictMode, err)
	}

	method := parsedValue[keys.method]
	timestr := parsedValue[keys.time]

	return NewParsedHTTPStat(uri, method, timestr, resTime, bodyBytes, status, traceID), nil
}

func normalizeURL(src *url.URL, queryString, qsIgnoreValues bool) string {
	if src.RawQuery == "" {
		return src.String()
	}

	u := *src // basic clone
	if !queryString {
		u.RawQuery = ""
		return u.String()
	}

	if qsIgnoreValues {
		values := u.Query()
		for q := range values {
			values.Set(q, "xxx")
		}
		u.RawQuery = values.Encode()
	} else {
		u.RawQuery = u.Query().Encode() // re-encode to sort queries
	}
	return u.String()
}

func errSkipReadLine(strictMode bool, err error) error {
	if strictMode {
		return err
	}

	return errors.SkipReadLineErr
}
