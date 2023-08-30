package stats

import (
	"embed"
	"fmt"
	"github.com/spaolacci/murmur3"
	"github.com/tetsuzawa/alp-trace/errors"
	"github.com/tetsuzawa/alp-trace/helpers"
	"github.com/tetsuzawa/alp-trace/options"
	"github.com/tetsuzawa/alp-trace/parsers"
	"math"
	"math/rand"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

//type traceHints struct {
//	values map[[]methodUriStatus]int
//	len    int
//	mu     sync.RWMutex
//}
//
//func newTraceHints() *traceHints {
//	return &traceHints{
//		values: make(map[string]int),
//	}
//}
//
//func (h *traceHints) loadOrStore(key string) int {
//	h.mu.Lock()
//	defer h.mu.Unlock()
//	_, ok := h.values[key]
//	if !ok {
//		h.values[key] = h.len
//		h.len++
//	}
//
//	return h.values[key]
//}

type TraceStats struct {
	hints                          *hints
	traceRequestDetailsMap         TraceRequestDetailsMap
	GlobalStat                     *GlobalStat
	ScenarioStats                  []*ScenarioStat
	useResponseTimePercentile      bool
	useRequestBodyBytesPercentile  bool
	useResponseBodyBytesPercentile bool
	filter                         *Filter
	options                        *options.Options
	sortOptions                    *SortOptions
	uriMatchingGroups              []*regexp.Regexp
}

// TraceRequestDetailsMap -> trace_id: [method1_uri1_, method2_uri2, ...]
type TraceRequestDetailsMap map[string][]*RequestDetail

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
		GlobalStat:                     newGlobalStat(useResTimePercentile, useRequestBodyBytesPercentile, useResponseBodyBytesPercentile),
		ScenarioStats:                  make([]*ScenarioStat, 0),
		useResponseTimePercentile:      useResTimePercentile,
		useResponseBodyBytesPercentile: useResponseBodyBytesPercentile,
	}
}

func (ts *TraceStats) AggregateTrace() {
	for traceID, requestDetails := range ts.traceRequestDetailsMap {
		//requestDetails := make([]*RequestDetail, 0, len(requestDetailss))

		// リクエストuriのリストを作成
		// query stringの除去が有効化の場合、除去する
		//for _, requestDetail := range requestDetails {
		//	uri := requestDetail.Uri
		//	if len(ts.uriMatchingGroups) > 0 {
		//		for _, re := range ts.uriMatchingGroups {
		//			if ok := re.Match([]byte(uri)); !ok {
		//				continue
		//			}
		//			pattern := re.String()
		//			uri = pattern
		//			break
		//		}
		//	}
		//	requestDetails = append(requestDetails, &RequestDetail{
		//		Uri:               uri,
		//		Method:            requestDetail.Method,
		//		Status:            requestDetail.Status,
		//		ResponseTime:      requestDetail.ResponseTime,
		//		RequestBodyBytes:  requestDetail.RequestBodyBytes,
		//		ResponseBodyBytes: requestDetail.ResponseBodyBytes,
		//		Pos:               requestDetail.Pos,
		//	})
		//}

		// keyの生成. ex: GET /foo/bar 200<br>POST /foo/bar 200
		//key := ""
		//for i := 0; i < len(uris); i++ {
		//	key += fmt.Sprintf("%s %s %d", requestDetails[i].Method, uris[i], requestDetails[i].Status)
		//	if i != len(uris)-1 {
		//		key += "<br>"
		//	}
		//}
		resultStatIDGenerator := murmur3.New32()
		for _, requestDetail := range requestDetails {
			resultStatIDPart := fmt.Sprintf("%s%s%d", requestDetail.Method, requestDetail.Uri, requestDetail.Status)
			resultStatIDGenerator.Write([]byte(resultStatIDPart))
		}
		resultStatID := fmt.Sprintf("%x", resultStatIDGenerator.Sum(nil))

		// 表示制限の数に至っていなければ追加
		idx := ts.hints.loadOrStore(resultStatID)
		if len(ts.ScenarioStats) <= idx {
			ts.ScenarioStats = append(ts.ScenarioStats, newTraceStat(resultStatID, requestDetails, ts.useResponseTimePercentile, ts.useRequestBodyBytesPercentile, ts.useResponseBodyBytesPercentile))
		}

		ts.GlobalStat.Set(requestDetails)
		ts.ScenarioStats[idx].Set(traceID, requestDetails)
	}
}

func (ts *ScenarioStat) UriWithOptions(decode bool) string {
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

//func (ts *TraceStats) ScenarioStats() []*ScenarioStat {
//	return ts.ScenarioStats
//}

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

	for _, s := range ts.ScenarioStats {
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

type ScenarioStat struct {
	ID string
	// todo 名前考える
	// ex: GET /foo/bar 200<br>POST /foo/bar 200
	TraceUriMethodStatus string
	Cnt                  int
	ResponseTime         *responseTime
	RequestBodyBytes     *bodyBytes
	ResponseBodyBytes    *bodyBytes
	//RequestDetails       []RequestDetail
	RequestDetailsStats []*RequestDetailStat

	TraceIDs    []string
	traceIDRand *rand.Rand
}

func newTraceStat(id string, requestDetails []*RequestDetail, useResTimePercentile, useRequestBodyBytesPercentile, useResponseBodyBytesPercentile bool) *ScenarioStat {
	rdss := make([]*RequestDetailStat, len(requestDetails))
	for i := range rdss {
		rdss[i] = newRequestDetailStat(requestDetails[i], useResTimePercentile, useRequestBodyBytesPercentile, useResponseBodyBytesPercentile)
	}
	return &ScenarioStat{
		ID:                  id,
		ResponseTime:        newResponseTime(useResTimePercentile),
		RequestBodyBytes:    newBodyBytes(useRequestBodyBytesPercentile),
		ResponseBodyBytes:   newBodyBytes(useResponseBodyBytesPercentile),
		RequestDetailsStats: rdss,
		TraceIDs:            make([]string, 0),
		traceIDRand:         rand.New(rand.NewSource(0)),
	}
}

func (ts *ScenarioStat) Set(traceID string, requestDetails []*RequestDetail) {
	restime := 0.0
	resBodyBytes := 0.0
	reqBodyBytes := 0.0
	// total response time, body bytesの計算
	for _, requestDetail := range requestDetails {
		restime += requestDetail.ResponseTime
		resBodyBytes += requestDetail.ResponseBodyBytes
		reqBodyBytes += requestDetail.RequestBodyBytes
	}

	ts.Cnt++
	ts.ResponseTime.Set(restime)
	ts.RequestBodyBytes.Set(reqBodyBytes)
	ts.ResponseBodyBytes.Set(resBodyBytes)
	for i := range ts.RequestDetailsStats {
		ts.RequestDetailsStats[i].Set(requestDetails[i])
	}
	ts.TraceIDs = append(ts.TraceIDs, traceID)
}

func (ts *ScenarioStat) Count() int {
	return ts.Cnt
}

func (ts *ScenarioStat) StrCount() string {
	return fmt.Sprint(ts.Cnt)
}

func (ts *ScenarioStat) MaxResponseTime() float64 {
	return ts.ResponseTime.Max
}

func (ts *ScenarioStat) MinResponseTime() float64 {
	return ts.ResponseTime.Min
}

func (ts *ScenarioStat) SumResponseTime() float64 {
	return ts.ResponseTime.Sum
}

func (ts *ScenarioStat) AvgResponseTime() float64 {
	return ts.ResponseTime.Avg(ts.Cnt)
}

func (ts *ScenarioStat) PNResponseTime(n int) float64 {
	return ts.ResponseTime.PN(ts.Cnt, n)
}

func (ts *ScenarioStat) StddevResponseTime() float64 {
	return ts.ResponseTime.Stddev(ts.Cnt)
}

// request
func (ts *ScenarioStat) MaxRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Max
}

func (ts *ScenarioStat) MinRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Min
}

func (ts *ScenarioStat) SumRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Sum
}

func (ts *ScenarioStat) AvgRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Avg(ts.Cnt)
}

func (ts *ScenarioStat) PNRequestBodyBytes(n int) float64 {
	return ts.RequestBodyBytes.PN(ts.Cnt, n)
}

func (ts *ScenarioStat) StddevRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Stddev(ts.Cnt)
}

// response
func (ts *ScenarioStat) MaxResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Max
}

func (ts *ScenarioStat) MinResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Min
}

func (ts *ScenarioStat) SumResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Sum
}

func (ts *ScenarioStat) AvgResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Avg(ts.Cnt)
}

func (ts *ScenarioStat) PNResponseBodyBytes(n int) float64 {
	return ts.RequestBodyBytes.PN(ts.Cnt, n)
}

func (ts *ScenarioStat) StddevResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Stddev(ts.Cnt)
}

func (ts *ScenarioStat) RandomTraceID() string {
	return ts.TraceIDs[ts.traceIDRand.Intn(len(ts.TraceIDs))]
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

	requestDetail := &RequestDetail{
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
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].Count() > ts.ScenarioStats[j].Count()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].Count() < ts.ScenarioStats[j].Count()
		})
	}
}

//func (ts *ScenarioStats) SortUri(reverse bool) {
//	if reverse {
//		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
//			return ts.ScenarioStats.ScenarioStats[i].Uri > ts.ScenarioStats.ScenarioStats[j].Uri
//		})
//	} else {
//		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
//			return ts.ScenarioStats.ScenarioStats[i].Uri < ts.ScenarioStats.ScenarioStats[j].Uri
//		})
//	}
//}
//
//func (ts *ScenarioStats) SortMethod(reverse bool) {
//	if reverse {
//		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
//			return ts.ScenarioStats.ScenarioStats[i].Method > ts.ScenarioStats.ScenarioStats[j].Method
//		})
//	} else {
//		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
//			return ts.ScenarioStats.ScenarioStats[i].Method < ts.ScenarioStats.ScenarioStats[j].Method
//		})
//	}
//}

func (ts *TraceStats) SortMaxResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MaxResponseTime() > ts.ScenarioStats[j].MaxResponseTime()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MaxResponseTime() < ts.ScenarioStats[j].MaxResponseTime()
		})
	}
}

func (ts *TraceStats) SortMinResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MinResponseTime() > ts.ScenarioStats[j].MinResponseTime()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MinResponseTime() < ts.ScenarioStats[j].MinResponseTime()
		})
	}
}

func (ts *TraceStats) SortSumResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].SumResponseTime() > ts.ScenarioStats[j].SumResponseTime()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].SumResponseTime() < ts.ScenarioStats[j].SumResponseTime()
		})
	}
}

func (ts *TraceStats) SortAvgResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].AvgResponseTime() > ts.ScenarioStats[j].AvgResponseTime()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].AvgResponseTime() < ts.ScenarioStats[j].AvgResponseTime()
		})
	}
}

func (ts *TraceStats) SortPNResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].PNResponseTime(ts.sortOptions.percentile) > ts.ScenarioStats[j].PNResponseTime(ts.sortOptions.percentile)
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].PNResponseTime(ts.sortOptions.percentile) < ts.ScenarioStats[j].PNResponseTime(ts.sortOptions.percentile)
		})
	}
}

func (ts *TraceStats) SortStddevResponseTime(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].StddevResponseTime() > ts.ScenarioStats[j].StddevResponseTime()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].StddevResponseTime() < ts.ScenarioStats[j].StddevResponseTime()
		})
	}
}

// request
func (ts *TraceStats) SortMaxRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MaxRequestBodyBytes() > ts.ScenarioStats[j].MaxRequestBodyBytes()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MaxRequestBodyBytes() < ts.ScenarioStats[j].MaxRequestBodyBytes()
		})
	}
}

func (ts *TraceStats) SortMinRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MinRequestBodyBytes() > ts.ScenarioStats[j].MinRequestBodyBytes()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MinRequestBodyBytes() < ts.ScenarioStats[j].MinRequestBodyBytes()
		})
	}
}

func (ts *TraceStats) SortSumRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].SumRequestBodyBytes() > ts.ScenarioStats[j].SumRequestBodyBytes()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].SumRequestBodyBytes() < ts.ScenarioStats[j].SumRequestBodyBytes()
		})
	}
}

func (ts *TraceStats) SortAvgRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].AvgRequestBodyBytes() > ts.ScenarioStats[j].AvgRequestBodyBytes()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].AvgRequestBodyBytes() < ts.ScenarioStats[j].AvgRequestBodyBytes()
		})
	}
}

func (ts *TraceStats) SortPNRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].PNRequestBodyBytes(ts.sortOptions.percentile) > ts.ScenarioStats[j].PNRequestBodyBytes(ts.sortOptions.percentile)
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].PNRequestBodyBytes(ts.sortOptions.percentile) < ts.ScenarioStats[j].PNRequestBodyBytes(ts.sortOptions.percentile)
		})
	}
}

func (ts *TraceStats) SortStddevRequestBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].StddevRequestBodyBytes() > ts.ScenarioStats[j].StddevRequestBodyBytes()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].StddevRequestBodyBytes() < ts.ScenarioStats[j].StddevRequestBodyBytes()
		})
	}
}

// response
func (ts *TraceStats) SortMaxResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MaxResponseBodyBytes() > ts.ScenarioStats[j].MaxResponseBodyBytes()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MaxResponseBodyBytes() < ts.ScenarioStats[j].MaxResponseBodyBytes()
		})
	}
}

func (ts *TraceStats) SortMinResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MinResponseBodyBytes() > ts.ScenarioStats[j].MinResponseBodyBytes()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].MinResponseBodyBytes() < ts.ScenarioStats[j].MinResponseBodyBytes()
		})
	}
}

func (ts *TraceStats) SortSumResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].SumResponseBodyBytes() > ts.ScenarioStats[j].SumResponseBodyBytes()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].SumResponseBodyBytes() < ts.ScenarioStats[j].SumResponseBodyBytes()
		})
	}
}

func (ts *TraceStats) SortAvgResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].AvgResponseBodyBytes() > ts.ScenarioStats[j].AvgResponseBodyBytes()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].AvgResponseBodyBytes() < ts.ScenarioStats[j].AvgResponseBodyBytes()
		})
	}
}

func (ts *TraceStats) SortPNResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].PNResponseBodyBytes(ts.sortOptions.percentile) > ts.ScenarioStats[j].PNResponseBodyBytes(ts.sortOptions.percentile)
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].PNResponseBodyBytes(ts.sortOptions.percentile) < ts.ScenarioStats[j].PNResponseBodyBytes(ts.sortOptions.percentile)
		})
	}
}

func (ts *TraceStats) SortStddevResponseBodyBytes(reverse bool) {
	if reverse {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].StddevResponseBodyBytes() > ts.ScenarioStats[j].StddevResponseBodyBytes()
		})
	} else {
		sort.Slice(ts.ScenarioStats, func(i, j int) bool {
			return ts.ScenarioStats[i].StddevResponseBodyBytes() < ts.ScenarioStats[j].StddevResponseBodyBytes()
		})
	}
}

// ==========================================================================
func (ts *TraceStats) widthRank() int {
	w := getIntWidth(float64(ts.options.Limit))
	if w < 4 {
		w = 4
	}
	return w
}

func (ts *TraceStats) DrawRankHeader() string {
	w := ts.widthRank()
	s := "Rank"
	return s + strings.Repeat(" ", w-len(s))
}

func (ts *TraceStats) DrawRankHR() string {
	w := ts.widthRank()
	return strings.Repeat("=", w)
}

func (ts *TraceStats) FormatRank(v int) string {
	w := ts.widthRank()
	f := fmt.Sprintf("%%%dd", w)
	return fmt.Sprintf(f, v+1)
}

func (ts *TraceStats) widthMethod() int {
	w := 5
	return w
}

func (ts *TraceStats) widthUri() int {
	maxLen := -1
	for _, s := range ts.ScenarioStats {
		for _, r := range s.RequestDetailsStats {
			if len(r.RequestDetail.Uri) > maxLen {
				maxLen = len(r.RequestDetail.Uri)
			}
		}
	}
	w := maxLen
	if w < 7 {
		w = 7
	}
	return w
}

func (ts *TraceStats) widthStatus() int {
	w := 3
	return w
}

func (ts *TraceStats) widthRequest() int {
	w := ts.widthMethod() + 1 + ts.widthUri() + 1 + ts.widthStatus()
	return w
}

func (ts *TraceStats) DrawRequestHeader() string {
	w := ts.widthRequest()
	s := "Request"
	return s + strings.Repeat(" ", w-len(s))
}

func (ts *TraceStats) DrawRequestHR() string {
	w := ts.widthRequest()
	return strings.Repeat("=", w)
}

func (ts *TraceStats) FormatRequest(method, uri string, status int) string {
	wm, wu, ws := ts.widthMethod(), ts.widthUri(), ts.widthStatus()
	f := fmt.Sprintf("%%-%ds %%-%ds %%%dd", wm, wu, ws)
	return fmt.Sprintf(f, method, uri, status)
}

func (ts *TraceStats) widthScenarioID() int {
	w := len(ts.ScenarioStats[len(ts.ScenarioStats)-1].ID)
	if w < 11 {
		w = 11
	}
	return w
}

func (ts *TraceStats) DrawScenarioIDHeader() string {
	w := ts.widthScenarioID()
	s := "Scenario ID"
	return s + strings.Repeat(" ", w-len(s))
}

func (ts *TraceStats) DrawScenarioIDHR() string {
	w := ts.widthScenarioID()
	return strings.Repeat("=", w)
}

func (ts *TraceStats) FormatScenarioID(v string) string {
	w := ts.widthScenarioID()
	f := fmt.Sprintf("%%-%ds", w)
	return fmt.Sprintf(f, v)
}

func (ts *TraceStats) widthSum() int {
	w := getIntWidth(ts.GlobalStat.ResponseTime.Sum)
	if w < 2 {
		w = 2
	}
	// Int.xxxx
	w += 2
	return w
}

func (ts *TraceStats) DrawSumHeader() string {
	// for percentages
	w := ts.widthSum() + 7
	s := "Sum"
	return s + strings.Repeat(" ", w-len(s))
}

func (ts *TraceStats) DrawSumHR() string {
	// for percentages
	w := ts.widthSum() + 7
	return strings.Repeat("=", w)
}

func (ts *TraceStats) FormatSum(v float64) string {
	w := ts.widthSum()
	f := fmt.Sprintf("%%%d.1f", w)
	return fmt.Sprintf(f, v)
}

func (ts *TraceStats) widthRate() int {
	w := 7
	return w
}

func (ts *TraceStats) DrawRateHeader() string {
	w := ts.widthRate()
	s := "Rate"
	return s + strings.Repeat(" ", w-len(s))
}

func (ts *TraceStats) DrawRateHR() string {
	w := ts.widthRate()
	return strings.Repeat("=", w)
}

func (ts *TraceStats) FormatRate(v float64) string {
	w := ts.widthRate()
	f := fmt.Sprintf("%%%d.0f", w)
	return fmt.Sprintf(f, v)
}

func (ts *TraceStats) widthMin() int {
	w := getIntWidth(ts.GlobalStat.ResponseTime.Min)
	if w < 2 {
		w = 2
	}
	// Int.xxxx
	w += 5
	return w
}

func (ts *TraceStats) DrawMinHeader() string {
	w := ts.widthMin()
	s := "Min"
	return s + strings.Repeat(" ", w-len(s))
}

func (ts *TraceStats) DrawMinHR() string {
	w := ts.widthMin()
	return strings.Repeat("=", w)
}

func (ts *TraceStats) FormatMin(v float64) string {
	w := ts.widthMin()
	f := fmt.Sprintf("%%%d.4f", w)
	return fmt.Sprintf(f, v)
}

func (ts *TraceStats) widthMax() int {
	w := getIntWidth(ts.GlobalStat.ResponseTime.Max)
	if w < 2 {
		w = 2
	}
	// Int.xxxx
	w += 5
	return w
}

func (ts *TraceStats) DrawMaxHeader() string {
	w := ts.widthMax()
	s := "Max"
	return s + strings.Repeat(" ", w-len(s))
}

func (ts *TraceStats) DrawMaxHR() string {
	w := ts.widthMax()
	return strings.Repeat("=", w)
}

func (ts *TraceStats) FormatMax(v float64) string {
	w := ts.widthMin()
	f := fmt.Sprintf("%%%d.4f", w)
	return fmt.Sprintf(f, v)
}

func (ts *TraceStats) widthCount() int {
	w := getIntWidth(float64(ts.GlobalStat.Cnt))
	if w < 5 {
		w = 5
	}
	return w
}

func (ts *TraceStats) FormatCount(v int) string {
	w := ts.widthCount()
	f := fmt.Sprintf("%%%dd", w)
	return fmt.Sprintf(f, v)
}

func (ts *TraceStats) DrawCountHeader() string {
	w := ts.widthCount()
	s := "Count"
	return s + strings.Repeat(" ", w-len(s))
}
func (ts *TraceStats) DrawCountHR() string {
	w := ts.widthCount()
	return strings.Repeat("=", w)
}

func (ts *TraceStats) widthAverage() int {
	//w := getIntWidth(ts.GlobalStat.ResponseTime.Avg(ts.GlobalStat.Cnt))
	//if w < 4 {
	//	w = 4
	//}
	w := 5
	return w
}

func (ts *TraceStats) DrawAverageHeader() string {
	w := ts.widthAverage()
	s := "Avg"
	return s + strings.Repeat(" ", w-len(s))
}

func (ts *TraceStats) DrawAverageHR() string {
	w := ts.widthAverage()
	return strings.Repeat("=", w)
}

func (ts *TraceStats) FormatAverage(v float64) string {
	w := ts.widthAverage()
	f := fmt.Sprintf("%%%d.2f", w)
	return fmt.Sprintf(f, v)
}

func (ts *TraceStats) widthRPCount() int {
	w := 7
	return w
}

func (ts *TraceStats) DrawRPCountHeader() string {
	w := ts.widthRPCount()
	s := "R/Count"
	return s + strings.Repeat(" ", w-len(s))
}

func (ts *TraceStats) DrawRPCountHR() string {
	w := ts.widthRPCount()
	return strings.Repeat("=", w)
}

func (ts *TraceStats) FormatRPCount(v float64) string {
	w := ts.widthRPCount()
	f := fmt.Sprintf("%%%d.4f", w)
	return fmt.Sprintf(f, v)
}

func (ts *TraceStats) widthP95() int {
	w := getIntWidth(ts.GlobalStat.ResponseTime.PN(ts.GlobalStat.Cnt, 95))
	if w < 2 {
		w = 2
	}
	// Int.xxxx
	w += 5
	return w
}

func (ts *TraceStats) DrawP95Header() string {
	w := ts.widthP95()
	s := "P95"
	return s + strings.Repeat(" ", w-len(s))
}

func (ts *TraceStats) DrawP95HR() string {
	w := ts.widthP95()
	return strings.Repeat("=", w)
}

func (ts *TraceStats) FormatP95(v float64) string {
	w := ts.widthMin()
	f := fmt.Sprintf("%%%d.2f", w)
	return fmt.Sprintf(f, v)
}

func (ts *TraceStats) widthMedian() int {
	w := getIntWidth(ts.GlobalStat.ResponseTime.PN(ts.GlobalStat.Cnt, 50))
	if w < 2 {
		w = 2
	}
	// Int.xxxx
	w += 5
	return w
}

func (ts *TraceStats) DrawMedianHeader() string {
	w := ts.widthMedian()
	s := "Median"
	return s + strings.Repeat(" ", w-len(s))
}

func (ts *TraceStats) DrawMedianHR() string {
	w := ts.widthMedian()
	return strings.Repeat("=", w)
}

func (ts *TraceStats) FormatMedian(v float64) string {
	w := ts.widthMedian()
	f := fmt.Sprintf("%%%d.2f", w)
	return fmt.Sprintf(f, v)
}

//go:embed templates
var fs embed.FS

var (
	reStmtName   = regexp.MustCompile(`^\*ast\.(.*)Stmt$`)
	rePrefixStmt = regexp.MustCompile(`\s.*`)
)

type OptLimit struct {
	count   *int
	percent *float64
}

func getIntWidth(v float64) int {
	w := 1

	abs := math.Abs(v)
	if abs != v {
		w++
	}
	if abs >= 1.0 {
		w += int(math.Log10(math.Abs(v)))
	}
	return w
}

// =============================================================================

type GlobalStat struct {
	Cnt               int
	ResponseTime      *responseTime
	RequestBodyBytes  *bodyBytes
	ResponseBodyBytes *bodyBytes
}

func newGlobalStat(useResTimePercentile, useRequestBodyBytesPercentile, useResponseBodyBytesPercentile bool) *GlobalStat {
	return &GlobalStat{
		ResponseTime:      newResponseTime(useResTimePercentile),
		RequestBodyBytes:  newBodyBytes(useRequestBodyBytesPercentile),
		ResponseBodyBytes: newBodyBytes(useResponseBodyBytesPercentile),
	}
}

func (ts *GlobalStat) Set(requestDetails []*RequestDetail) {
	for _, requestDetail := range requestDetails {
		ts.Cnt++
		ts.ResponseTime.Set(requestDetail.ResponseTime)
		ts.RequestBodyBytes.Set(requestDetail.RequestBodyBytes)
		ts.ResponseBodyBytes.Set(requestDetail.ResponseBodyBytes)
	}
}

type RequestDetailStat struct {
	// todo 名前考える
	// ex: GET /foo/bar 200<br>POST /foo/bar 200
	RequestDetail     *RequestDetail
	Cnt               int
	ResponseTime      *responseTime
	RequestBodyBytes  *bodyBytes
	ResponseBodyBytes *bodyBytes
}

func newRequestDetailStat(requestDetail *RequestDetail, useResTimePercentile, useRequestBodyBytesPercentile, useResponseBodyBytesPercentile bool) *RequestDetailStat {
	return &RequestDetailStat{
		RequestDetail:     requestDetail,
		ResponseTime:      newResponseTime(useResTimePercentile),
		RequestBodyBytes:  newBodyBytes(useRequestBodyBytesPercentile),
		ResponseBodyBytes: newBodyBytes(useResponseBodyBytesPercentile),
	}
}

func (ts *RequestDetailStat) Set(requestDetail *RequestDetail) {
	ts.Cnt++
	ts.ResponseTime.Set(requestDetail.ResponseTime)
	ts.RequestBodyBytes.Set(requestDetail.RequestBodyBytes)
	ts.ResponseBodyBytes.Set(requestDetail.ResponseBodyBytes)
}

func (ts *RequestDetailStat) Count() int {
	return ts.Cnt
}

func (ts *RequestDetailStat) StrCount() string {
	return fmt.Sprint(ts.Cnt)
}

func (ts *RequestDetailStat) MaxResponseTime() float64 {
	return ts.ResponseTime.Max
}

func (ts *RequestDetailStat) MinResponseTime() float64 {
	return ts.ResponseTime.Min
}

func (ts *RequestDetailStat) SumResponseTime() float64 {
	return ts.ResponseTime.Sum
}

func (ts *RequestDetailStat) AvgResponseTime() float64 {
	return ts.ResponseTime.Avg(ts.Cnt)
}

func (ts *RequestDetailStat) PNResponseTime(n int) float64 {
	return ts.ResponseTime.PN(ts.Cnt, n)
}

func (ts *RequestDetailStat) StddevResponseTime() float64 {
	return ts.ResponseTime.Stddev(ts.Cnt)
}

// request
func (ts *RequestDetailStat) MaxRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Max
}

func (ts *RequestDetailStat) MinRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Min
}

func (ts *RequestDetailStat) SumRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Sum
}

func (ts *RequestDetailStat) AvgRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Avg(ts.Cnt)
}

func (ts *RequestDetailStat) PNRequestBodyBytes(n int) float64 {
	return ts.RequestBodyBytes.PN(ts.Cnt, n)
}

func (ts *RequestDetailStat) StddevRequestBodyBytes() float64 {
	return ts.RequestBodyBytes.Stddev(ts.Cnt)
}

// response
func (ts *RequestDetailStat) MaxResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Max
}

func (ts *RequestDetailStat) MinResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Min
}

func (ts *RequestDetailStat) SumResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Sum
}

func (ts *RequestDetailStat) AvgResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Avg(ts.Cnt)
}

func (ts *RequestDetailStat) PNResponseBodyBytes(n int) float64 {
	return ts.RequestBodyBytes.PN(ts.Cnt, n)
}

func (ts *RequestDetailStat) StddevResponseBodyBytes() float64 {
	return ts.RequestBodyBytes.Stddev(ts.Cnt)
}

// ==============================
