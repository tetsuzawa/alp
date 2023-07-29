package stats

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"

	"github.com/tetsuzawa/alp-trace/errors"
	"github.com/tetsuzawa/alp-trace/helpers"
	"github.com/tetsuzawa/alp-trace/options"
	"github.com/tetsuzawa/alp-trace/parsers"
)

type TraceStats struct {
	hints                          *hints
	traceRequestDetailsMap         TraceRequestDetailsMap
	stats                          traceStats
	useResponseTimePercentile      bool
	useRequestBodyBytesPercentile  bool
	useResponseBodyBytesPercentile bool
	filter                         *Filter
	options                        *options.Options
	sortOptions                    *SortOptions
	uriMatchingGroups              []*regexp.Regexp
}

// TraceRequestDetailsMap -> trace_id: [method1_uri1_, method2_uri2, ...]
type TraceRequestDetailsMap map[string][]RequestDetail

type RequestDetail struct {
	Uri               string
	Method            string
	Status            int
	ResponseTime      float64
	RequestBodyBytes  float64
	ResponseBodyBytes float64
	Pos               int
}

func NewTraceStats(useResTimePercentile, useRequestBodyBytesPercentile, useResponseBodyBytesPercentile bool) *TraceStats {
	return &TraceStats{
		hints:                          newHints(),
		traceRequestDetailsMap:         make(TraceRequestDetailsMap),
		stats:                          make([]*TraceStat, 0),
		useResponseTimePercentile:      useResTimePercentile,
		useResponseBodyBytesPercentile: useResponseBodyBytesPercentile,
	}
}

func (ts *TraceStats) AggregateTrace() {
	for traceID, requestDetails := range ts.traceRequestDetailsMap {
		uris := make([]string, 0, len(requestDetails))

		// リクエストuriのリストを作成
		// query stringの除去が有効化の場合、除去する
		for _, requestDetail := range requestDetails {
			uri := requestDetail.Uri
			if len(ts.uriMatchingGroups) > 0 {
				for _, re := range ts.uriMatchingGroups {
					if ok := re.Match([]byte(uri)); ok {
						pattern := re.String()
						uri = pattern
						break
					}
				}
			}
			uris = append(uris, uri)
		}

		// keyの生成. ex: uri1__method1__status1::uri2__method2__status2::uri3__method3__status3
		key := ""
		for i := 0; i < len(uris); i++ {
			key += fmt.Sprintf("%s %s %d", requestDetails[i].Method, uris[i], requestDetails[i].Status)
			if i != len(uris)-1 {
				key += "<br>"
			}
		}

		idx := ts.hints.loadOrStore(key)
		if idx >= len(ts.stats) {
			ts.stats = append(ts.stats, newTraceStat(key, ts.useResponseTimePercentile, ts.useRequestBodyBytesPercentile, ts.useResponseBodyBytesPercentile))
		}

		restime := 0.0
		resBodyBytes := 0.0
		reqBodyBytes := 0.0
		// total response time, body bytesの計算
		for i := 0; i < len(uris); i++ {
			restime += requestDetails[i].ResponseTime
			resBodyBytes += requestDetails[i].ResponseBodyBytes
			reqBodyBytes += requestDetails[i].RequestBodyBytes
		}
		ts.stats[idx].Set(traceID, restime, resBodyBytes, reqBodyBytes)
	}
}

func (ts *TraceStat) UriWithOptions(decode bool) string {
	if !decode {
		return ts.TraceUriMethodStatus
	}

	u, err := url.Parse(ts.TraceUriMethodStatus)
	if err != nil {
		return ts.TraceUriMethodStatus
	}

	if u.RawQuery == "" {
		unescaped, _ := url.PathUnescape(u.EscapedPath())
		return unescaped
	}

	unescaped, _ := url.PathUnescape(u.EscapedPath())
	decoded, _ := url.QueryUnescape(u.Query().Encode())

	return fmt.Sprintf("%s?%s", unescaped, decoded)
}

func (ts *TraceStats) Stats() []*TraceStat {
	return ts.stats
}

func (ts *TraceStats) CountUris() int {
	return ts.hints.len
}

func (ts *TraceStats) SetOptions(options *options.Options) {
	ts.options = options
}

func (ts *TraceStats) SetSortOptions(options *SortOptions) {
	ts.sortOptions = options
}

func (ts *TraceStats) SetURIMatchingGroups(groups []string) error {
	uriGroups, err := helpers.CompileUriMatchingGroups(groups)
	if err != nil {
		return err
	}

	ts.uriMatchingGroups = uriGroups

	return nil
}

func (ts *TraceStats) InitFilter(options *options.Options) error {
	ts.filter = NewFilter(options)
	return ts.filter.Init()
}

func (ts *TraceStats) DoFilter(pstat *parsers.ParsedHTTPStat) (bool, error) {
	err := ts.filter.Do(pstat)
	if err == errors.SkipReadLineErr {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func (ts *TraceStats) CountAll() map[string]int {
	counts := make(map[string]int, 6)

	for _, s := range ts.stats {
		counts["count"] += s.Cnt
	}

	return counts
}

func (ts *TraceStats) SortWithOptions() {
	ts.Sort(ts.sortOptions, ts.options.Reverse)
}

// TODO sortedSliceを作って、RequestDetail追加時にソートする
// 遅延ソートでもいいかもしれない

// HTTPStatsの拡張

type TraceStat struct {
	// todo 名前考える
	// key. ex: uri1__method1__status1::uri2__method2__status2::uri3__method3__status3
	TraceUriMethodStatus string
	Cnt                  int
	ResponseTime         *responseTime
	RequestBodyBytes     *bodyBytes
	ResponseBodyBytes    *bodyBytes
	TraceIDs             []string
}

type traceStats []*TraceStat

func newTraceStat(traceUrlMethodStatus string, useResTimePercentile, useRequestBodyBytesPercentile, useResponseBodyBytesPercentile bool) *TraceStat {
	return &TraceStat{
		TraceUriMethodStatus: traceUrlMethodStatus,
		ResponseTime:         newResponseTime(useResTimePercentile),
		RequestBodyBytes:     newBodyBytes(useRequestBodyBytesPercentile),
		ResponseBodyBytes:    newBodyBytes(useResponseBodyBytesPercentile),
		TraceIDs:             make([]string, 0),
	}
}

func (ts *TraceStat) Set(traceID string, restime, reqBodyBytes, resBodyBytes float64) {
	ts.Cnt++
	ts.ResponseTime.Set(restime)
	ts.RequestBodyBytes.Set(reqBodyBytes)
	ts.ResponseBodyBytes.Set(resBodyBytes)
	ts.TraceIDs = append(ts.TraceIDs, traceID)
}

func (ts *TraceStat) Count() int {
	return ts.Cnt
}

func (ts *TraceStat) StrCount() string {
	return fmt.Sprint(ts.Cnt)
}

func (ts *TraceStat) MaxResponseTime() float64 {
	return ts.ResponseTime.Max
}

func (ts *TraceStat) MinResponseTime() float64 {
	return ts.ResponseTime.Min
}

func (ts *TraceStat) SumResponseTime() float64 {
	return ts.ResponseTime.Sum
}

func (ts *TraceStat) AvgResponseTime() float64 {
	return ts.ResponseTime.Avg(ts.Cnt)
}

func (ts *TraceStat) PNResponseTime(n int) float64 {
	return ts.ResponseTime.PN(ts.Cnt, n)
}

func (ts *TraceStat) StddevResponseTime() float64 {
	return ts.ResponseTime.Stddev(ts.Cnt)
}

// request
func (ts *TraceStat) MaxRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Max
}

func (ts *TraceStat) MinRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Min
}

func (ts *TraceStat) SumRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Sum
}

func (ts *TraceStat) AvgRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Avg(ts.Cnt)
}

func (ts *TraceStat) PNRequestBodyBytes(n int) float64 {
	return ts.RequestBodyBytes.PN(ts.Cnt, n)
}

func (ts *TraceStat) StddevRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Stddev(ts.Cnt)
}

// response
func (ts *TraceStat) MaxResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Max
}

func (ts *TraceStat) MinResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Min
}

func (ts *TraceStat) SumResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Sum
}

func (ts *TraceStat) AvgResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Avg(ts.Cnt)
}

func (ts *TraceStat) PNResponseBodyBytes(n int) float64 {
	return ts.RequestBodyBytes.PN(ts.Cnt, n)
}

func (ts *TraceStat) StddevResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Stddev(ts.Cnt)
}

func (ts *TraceStats) AppendTrace(traceID, uri, method string, status int, restime, resBodyBytes, reqBodyBytes float64, pos int) {
	if len(ts.uriMatchingGroups) > 0 {
		for _, re := range ts.uriMatchingGroups {
			if ok := re.Match([]byte(uri)); ok {
				pattern := re.String()
				uri = pattern
				break
			}
		}
	}

	requestDetail := RequestDetail{
		Uri:               uri,
		Method:            method,
		Status:            status,
		ResponseTime:      restime,
		RequestBodyBytes:  resBodyBytes,
		ResponseBodyBytes: reqBodyBytes,
		Pos:               pos,
	}
	ts.traceRequestDetailsMap[traceID] = append(ts.traceRequestDetailsMap[traceID], requestDetail)
}

func (ts *TraceStats) Sort(sortOptions *SortOptions, reverse bool) {
	switch sortOptions.sortType {
	case SortCount:
		ts.SortCount(reverse)
	//case SortUri:
	//	ts.SortUri(reverse)
	//case SortMethod:
	//	ts.SortMethod(reverse)
	// response time
	case SortMaxResponseTime:
		ts.SortMaxResponseTime(reverse)
	case SortMinResponseTime:
		ts.SortMinResponseTime(reverse)
	case SortSumResponseTime:
		ts.SortSumResponseTime(reverse)
	case SortAvgResponseTime:
		ts.SortAvgResponseTime(reverse)
	case SortPNResponseTime:
		ts.SortPNResponseTime(reverse)
	case SortStddevResponseTime:
		ts.SortStddevResponseTime(reverse)
	// request body bytes
	case SortMaxRequestBodyBytes:
		ts.SortMaxRequestBodyBytes(reverse)
	case SortMinRequestBodyBytes:
		ts.SortMinRequestBodyBytes(reverse)
	case SortSumRequestBodyBytes:
		ts.SortSumRequestBodyBytes(reverse)
	case SortAvgRequestBodyBytes:
		ts.SortAvgRequestBodyBytes(reverse)
	case SortPNRequestBodyBytes:
		ts.SortPNRequestBodyBytes(reverse)
	case SortStddevRequestBodyBytes:
		ts.SortStddevRequestBodyBytes(reverse)
	// response body bytes
	case SortMaxResponseBodyBytes:
		ts.SortMaxResponseBodyBytes(reverse)
	case SortMinResponseBodyBytes:
		ts.SortMinResponseBodyBytes(reverse)
	case SortSumResponseBodyBytes:
		ts.SortSumResponseBodyBytes(reverse)
	case SortAvgResponseBodyBytes:
		ts.SortAvgResponseBodyBytes(reverse)
	case SortPNResponseBodyBytes:
		ts.SortPNResponseBodyBytes(reverse)
	case SortStddevResponseBodyBytes:
		ts.SortStddevResponseBodyBytes(reverse)
	default:
		ts.SortCount(reverse)
	}
}

func (ts *TraceStats) SortCount(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].Count() > ts.stats[j].Count()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].Count() < ts.stats[j].Count()
		})
	}
}

//func (ts *stats) SortUri(reverse bool) {
//	if reverse {
//		sort.Slice(ts.stats, func(i, j int) bool {
//			return ts.stats[i].Uri > ts.stats[j].Uri
//		})
//	} else {
//		sort.Slice(ts.stats, func(i, j int) bool {
//			return ts.stats[i].Uri < ts.stats[j].Uri
//		})
//	}
//}
//
//func (ts *stats) SortMethod(reverse bool) {
//	if reverse {
//		sort.Slice(ts.stats, func(i, j int) bool {
//			return ts.stats[i].Method > ts.stats[j].Method
//		})
//	} else {
//		sort.Slice(ts.stats, func(i, j int) bool {
//			return ts.stats[i].Method < ts.stats[j].Method
//		})
//	}
//}

func (ts *TraceStats) SortMaxResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MaxResponseTime() > ts.stats[j].MaxResponseTime()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MaxResponseTime() < ts.stats[j].MaxResponseTime()
		})
	}
}

func (ts *TraceStats) SortMinResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MinResponseTime() > ts.stats[j].MinResponseTime()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MinResponseTime() < ts.stats[j].MinResponseTime()
		})
	}
}

func (ts *TraceStats) SortSumResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].SumResponseTime() > ts.stats[j].SumResponseTime()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].SumResponseTime() < ts.stats[j].SumResponseTime()
		})
	}
}

func (ts *TraceStats) SortAvgResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].AvgResponseTime() > ts.stats[j].AvgResponseTime()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].AvgResponseTime() < ts.stats[j].AvgResponseTime()
		})
	}
}

func (ts *TraceStats) SortPNResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].PNResponseTime(ts.sortOptions.percentile) > ts.stats[j].PNResponseTime(ts.sortOptions.percentile)
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].PNResponseTime(ts.sortOptions.percentile) < ts.stats[j].PNResponseTime(ts.sortOptions.percentile)
		})
	}
}

func (ts *TraceStats) SortStddevResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].StddevResponseTime() > ts.stats[j].StddevResponseTime()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].StddevResponseTime() < ts.stats[j].StddevResponseTime()
		})
	}
}

// request
func (ts *TraceStats) SortMaxRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MaxRequestBodyBytes() > ts.stats[j].MaxRequestBodyBytes()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MaxRequestBodyBytes() < ts.stats[j].MaxRequestBodyBytes()
		})
	}
}

func (ts *TraceStats) SortMinRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MinRequestBodyBytes() > ts.stats[j].MinRequestBodyBytes()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MinRequestBodyBytes() < ts.stats[j].MinRequestBodyBytes()
		})
	}
}

func (ts *TraceStats) SortSumRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].SumRequestBodyBytes() > ts.stats[j].SumRequestBodyBytes()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].SumRequestBodyBytes() < ts.stats[j].SumRequestBodyBytes()
		})
	}
}

func (ts *TraceStats) SortAvgRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].AvgRequestBodyBytes() > ts.stats[j].AvgRequestBodyBytes()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].AvgRequestBodyBytes() < ts.stats[j].AvgRequestBodyBytes()
		})
	}
}

func (ts *TraceStats) SortPNRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].PNRequestBodyBytes(ts.sortOptions.percentile) > ts.stats[j].PNRequestBodyBytes(ts.sortOptions.percentile)
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].PNRequestBodyBytes(ts.sortOptions.percentile) < ts.stats[j].PNRequestBodyBytes(ts.sortOptions.percentile)
		})
	}
}

func (ts *TraceStats) SortStddevRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].StddevRequestBodyBytes() > ts.stats[j].StddevRequestBodyBytes()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].StddevRequestBodyBytes() < ts.stats[j].StddevRequestBodyBytes()
		})
	}
}

// response
func (ts *TraceStats) SortMaxResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MaxResponseBodyBytes() > ts.stats[j].MaxResponseBodyBytes()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MaxResponseBodyBytes() < ts.stats[j].MaxResponseBodyBytes()
		})
	}
}

func (ts *TraceStats) SortMinResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MinResponseBodyBytes() > ts.stats[j].MinResponseBodyBytes()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].MinResponseBodyBytes() < ts.stats[j].MinResponseBodyBytes()
		})
	}
}

func (ts *TraceStats) SortSumResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].SumResponseBodyBytes() > ts.stats[j].SumResponseBodyBytes()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].SumResponseBodyBytes() < ts.stats[j].SumResponseBodyBytes()
		})
	}
}

func (ts *TraceStats) SortAvgResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].AvgResponseBodyBytes() > ts.stats[j].AvgResponseBodyBytes()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].AvgResponseBodyBytes() < ts.stats[j].AvgResponseBodyBytes()
		})
	}
}

func (ts *TraceStats) SortPNResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].PNResponseBodyBytes(ts.sortOptions.percentile) > ts.stats[j].PNResponseBodyBytes(ts.sortOptions.percentile)
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].PNResponseBodyBytes(ts.sortOptions.percentile) < ts.stats[j].PNResponseBodyBytes(ts.sortOptions.percentile)
		})
	}
}

func (ts *TraceStats) SortStddevResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].StddevResponseBodyBytes() > ts.stats[j].StddevResponseBodyBytes()
		})
	} else {
		sort.Slice(ts.stats, func(i, j int) bool {
			return ts.stats[i].StddevResponseBodyBytes() < ts.stats[j].StddevResponseBodyBytes()
		})
	}
}
