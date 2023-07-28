package stats

import (
	"fmt"
	"io"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/tkuchiki/alp/helpers"
	"github.com/tkuchiki/alp/html"
)

func traceKeywords(percentiles []int) []string {
	s1 := []string{
		"count",
		"uri_method_status",
		"min",
		"max",
		"sum",
		"avg",
	}

	s2 := []string{
		"stddev",
		"min_body",
		"max_body",
		"sum_body",
		"avg_body",
	}

	sp := make([]string, 0, len(percentiles))
	for _, p := range percentiles {
		sp = append(sp, fmt.Sprintf("p%d", p))
	}

	s := make([]string, 0, len(s1)+len(s2)+len(sp))
	s = append(s, s1...)
	s = append(s, sp...)
	s = append(s, s2...)

	return s
}

func traceDefaultHeaders(percentiles []int) []string {
	s1 := []string{
		"Count",
		"UriMethodStatus",
		"Min",
		"Max",
		"Sum",
		"Avg",
	}

	s2 := []string{
		"Stddev",
		"Min(Body)",
		"Max(Body)",
		"Sum(Body)",
		"Avg(Body)",
	}

	sp := make([]string, 0, len(percentiles))
	for _, p := range percentiles {
		sp = append(sp, fmt.Sprintf("P%d", p))
	}

	s := make([]string, 0, len(s1)+len(s2)+len(sp))
	s = append(s, s1...)
	s = append(s, sp...)
	s = append(s, s2...)

	return s
}

func traceHeadersMap(percentiles []int) map[string]string {
	headers := map[string]string{
		"count":             "Count",
		"uri_method_status": "UriMethodStatus",
		"min":               "Min",
		"max":               "Max",
		"sum":               "Sum",
		"avg":               "Avg",
		"stddev":            "Stddev",
		"min_body":          "Min(Body)",
		"max_body":          "Max(Body)",
		"sum_body":          "Sum(Body)",
		"avg_body":          "Avg(Body)",
	}

	for _, p := range percentiles {
		key := fmt.Sprintf("p%d", p)
		val := fmt.Sprintf("P%d", p)
		headers[key] = val
	}

	return headers
}

type TracePrintOptions struct {
	noHeaders       bool
	showFooters     bool
	decodeUri       bool
	paginationLimit int
}

func NewTracePrintOptions(noHeaders, showFooters, decodeUri bool, paginationLimit int) *TracePrintOptions {
	return &TracePrintOptions{
		noHeaders:       noHeaders,
		showFooters:     showFooters,
		decodeUri:       decodeUri,
		paginationLimit: paginationLimit,
	}
}

type TracePrinter struct {
	keywords     []string
	format       string
	percentiles  []int
	printOptions *TracePrintOptions
	headers      []string
	headersMap   map[string]string
	writer       io.Writer
	all          bool
}

func NewTracePrinter(w io.Writer, val, format string, percentiles []int, printOptions *TracePrintOptions) *TracePrinter {
	p := &TracePrinter{
		format:       format,
		percentiles:  percentiles,
		headersMap:   traceHeadersMap(percentiles),
		writer:       w,
		printOptions: printOptions,
	}

	if val == "all" {
		p.keywords = traceKeywords(percentiles)
		p.headers = traceDefaultHeaders(percentiles)
		p.all = true
	} else {
		p.keywords = helpers.SplitCSV(val)
		for _, key := range p.keywords {
			p.headers = append(p.headers, p.headersMap[key])
			if key == "all" {
				p.keywords = traceKeywords(percentiles)
				p.headers = traceDefaultHeaders(percentiles)
				p.all = true
				break
			}
		}
	}

	return p
}

func (p *TracePrinter) Validate() error {
	if p.all {
		return nil
	}

	invalids := make([]string, 0)
	for _, key := range p.keywords {
		if _, ok := p.headersMap[key]; !ok {
			invalids = append(invalids, key)
		}
	}

	if len(invalids) > 0 {
		return fmt.Errorf("invalid keywords: %s", strings.Join(invalids, ","))
	}

	return nil
}

func (p *TracePrinter) GenerateTraceLine(s *TraceStat, quoteUri bool) []string {
	keyLen := len(p.keywords)
	line := make([]string, 0, keyLen)

	for i := 0; i < keyLen; i++ {
		switch p.keywords[i] {
		case "count":
			line = append(line, s.StrCount())
		case "uri_method_status":
			uriMethodStatus := s.UriWithOptions(p.printOptions.decodeUri)
			if quoteUri && strings.Contains(s.TraceUriMethodStatus, ",") {
				uriMethodStatus = fmt.Sprintf(`"%s"`, s.TraceUriMethodStatus)
			}
			line = append(line, uriMethodStatus)
		case "min":
			line = append(line, round(s.MinResponseTime()))
		case "max":
			line = append(line, round(s.MaxResponseTime()))
		case "sum":
			line = append(line, round(s.SumResponseTime()))
		case "avg":
			line = append(line, round(s.AvgResponseTime()))
		case "stddev":
			line = append(line, round(s.StddevResponseTime()))
		case "min_body":
			line = append(line, round(s.MinResponseBodyBytes()))
		case "max_body":
			line = append(line, round(s.MaxResponseBodyBytes()))
		case "sum_body":
			line = append(line, round(s.SumResponseBodyBytes()))
		case "avg_body":
			line = append(line, round(s.AvgResponseBodyBytes()))
		default: // percentile
			var n int
			_, err := fmt.Sscanf(p.keywords[i], "p%d", &n)
			if err != nil {
				continue
			}
			line = append(line, round(s.PNResponseTime(n)))
		}
	}

	return line
}

func (p *TracePrinter) GenerateTraceLineWithDiff(from, to *TraceStat, quoteUri bool) []string {
	keyLen := len(p.keywords)
	line := make([]string, 0, keyLen)

	differ := NewTraceDiffer(from, to)

	for i := 0; i < keyLen; i++ {
		switch p.keywords[i] {
		case "count":
			line = append(line, formattedLineWithDiff(to.StrCount(), differ.DiffCnt()))
		case "uri_method_status":
			uriMethodStatus := to.UriWithOptions(p.printOptions.decodeUri)
			if quoteUri && strings.Contains(to.TraceUriMethodStatus, ",") {
				uriMethodStatus = fmt.Sprintf(`"%s"`, to.TraceUriMethodStatus)
			}
			line = append(line, uriMethodStatus)
		case "min":
			line = append(line, formattedLineWithDiff(round(to.MinResponseTime()), differ.DiffMinResponseTime()))
		case "max":
			line = append(line, formattedLineWithDiff(round(to.MaxResponseTime()), differ.DiffMaxResponseTime()))
		case "sum":
			line = append(line, formattedLineWithDiff(round(to.SumResponseTime()), differ.DiffSumResponseTime()))
		case "avg":
			line = append(line, formattedLineWithDiff(round(to.AvgResponseTime()), differ.DiffAvgResponseTime()))
		case "stddev":
			line = append(line, formattedLineWithDiff(round(to.StddevResponseTime()), differ.DiffStddevResponseTime()))
		case "min_body":
			line = append(line, formattedLineWithDiff(round(to.MinResponseBodyBytes()), differ.DiffMinResponseBodyBytes()))
		case "max_body":
			line = append(line, formattedLineWithDiff(round(to.MaxResponseBodyBytes()), differ.DiffMaxResponseBodyBytes()))
		case "sum_body":
			line = append(line, formattedLineWithDiff(round(to.SumResponseBodyBytes()), differ.DiffSumResponseBodyBytes()))
		case "avg_body":
			line = append(line, formattedLineWithDiff(round(to.AvgResponseBodyBytes()), differ.DiffAvgResponseBodyBytes()))
		default: // percentile
			var n int
			_, err := fmt.Sscanf(p.keywords[i], "p%d", &n)
			if err != nil {
				continue
			}
			line = append(line, formattedLineWithDiff(round(to.PNResponseTime(n)), differ.DiffPNResponseTime(n)))
		}
	}

	return line
}

func (p *TracePrinter) GenerateTraceFooter(counts map[string]int) []string {
	keyLen := len(p.keywords)
	line := make([]string, 0, keyLen)

	for i := 0; i < keyLen; i++ {
		switch p.keywords[i] {
		case "count":
			line = append(line, fmt.Sprint(counts["count"]))
		default:
			line = append(line, "")
		}
	}

	return line
}

func (p *TracePrinter) GenerateTraceFooterWithDiff(countsFrom, countsTo map[string]int) []string {
	keyLen := len(p.keywords)
	line := make([]string, 0, keyLen)
	counts := DiffCountAll(countsFrom, countsTo)

	for i := 0; i < keyLen; i++ {
		switch p.keywords[i] {
		case "count":
			line = append(line, formattedLineWithDiff(fmt.Sprint(countsTo["count"]), counts["count"]))
		default:
			line = append(line, "")
		}
	}

	return line
}

func (p *TracePrinter) SetFormat(format string) {
	p.format = format
}

func (p *TracePrinter) SetHeaders(headers []string) {
	p.headers = headers
}

func (p *TracePrinter) SetWriter(w io.Writer) {
	p.writer = w
}

func (p *TracePrinter) Print(ts, tsTo *TraceStats) {
	switch p.format {
	case "table":
		p.printTraceTable(ts, tsTo)
	case "md", "markdown":
		p.printTraceMarkdown(ts, tsTo)
	case "tsv":
		p.printTraceTSV(ts, tsTo)
	case "csv":
		p.printTraceCSV(ts, tsTo)
	case "html":
		p.printTraceHTML(ts, tsTo)
	}
}

//func round(num float64) string {
//	return fmt.Sprintf("%.3f", num)
//}

func findTraceStatFrom(tsFrom *TraceStats, tsTo *TraceStat) *TraceStat {
	for _, sFrom := range tsFrom.stats {
		if sFrom.TraceUriMethodStatus == tsTo.TraceUriMethodStatus {
			return sFrom
		}
	}
	return nil
}

func (p *TracePrinter) printTraceTable(tsFrom, tsTo *TraceStats) {
	table := tablewriter.NewWriter(p.writer)
	table.SetHeader(p.headers)
	if tsTo == nil {
		for _, s := range tsFrom.stats {
			data := p.GenerateTraceLine(s, false)
			table.Append(data)
		}
	} else {
		for _, to := range tsTo.stats {
			from := findTraceStatFrom(tsFrom, to)

			var data []string
			if from == nil {
				data = p.GenerateTraceLine(to, false)
			} else {
				data = p.GenerateTraceLineWithDiff(from, to, false)
			}
			table.Append(data)
		}
	}

	if p.printOptions.showFooters {
		var footer []string
		if tsTo == nil {
			footer = p.GenerateTraceFooter(tsFrom.CountAll())
		} else {
			footer = p.GenerateTraceFooterWithDiff(tsFrom.CountAll(), tsTo.CountAll())
		}
		table.SetFooter(footer)
		table.SetFooterAlignment(tablewriter.ALIGN_LEFT)
	}

	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.Render()
}

func (p *TracePrinter) printTraceMarkdown(tsFrom, tsTo *TraceStats) {
	table := tablewriter.NewWriter(p.writer)
	table.SetHeader(p.headers)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	if tsTo == nil {
		for _, s := range tsFrom.stats {
			data := p.GenerateTraceLine(s, false)
			table.Append(data)
		}
	} else {
		for _, to := range tsTo.stats {
			from := findTraceStatFrom(tsFrom, to)

			var data []string
			if from == nil {
				data = p.GenerateTraceLine(to, false)
			} else {
				data = p.GenerateTraceLineWithDiff(from, to, false)
			}
			table.Append(data)
		}
	}

	if p.printOptions.showFooters {
		var footer []string
		if tsTo == nil {
			footer = p.GenerateTraceFooter(tsFrom.CountAll())
		} else {
			footer = p.GenerateTraceFooterWithDiff(tsFrom.CountAll(), tsTo.CountAll())
		}
		table.Append(footer)
	}

	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.Render()
}

func (p *TracePrinter) printTraceTSV(tsFrom, tsTo *TraceStats) {
	if !p.printOptions.noHeaders {
		fmt.Println(strings.Join(p.headers, "\t"))
	}

	var data []string
	if tsTo == nil {
		for _, s := range tsFrom.stats {
			data = p.GenerateTraceLine(s, false)
			fmt.Println(strings.Join(data, "\t"))
		}
	} else {
		for _, to := range tsTo.stats {
			from := findTraceStatFrom(tsFrom, to)

			if from == nil {
				data = p.GenerateTraceLine(to, false)
			} else {
				data = p.GenerateTraceLineWithDiff(from, to, false)
			}
			fmt.Println(strings.Join(data, "\t"))
		}
	}
}

func (p *TracePrinter) printTraceCSV(tsFrom, tsTo *TraceStats) {
	if !p.printOptions.noHeaders {
		fmt.Println(strings.Join(p.headers, ","))
	}

	var data []string
	if tsTo == nil {
		for _, s := range tsFrom.stats {
			data = p.GenerateTraceLine(s, true)
			fmt.Println(strings.Join(data, ","))
		}
	} else {
		for _, to := range tsTo.stats {
			from := findTraceStatFrom(tsFrom, to)

			if from == nil {
				data = p.GenerateTraceLine(to, false)
			} else {
				data = p.GenerateTraceLineWithDiff(from, to, false)
			}
			fmt.Println(strings.Join(data, ","))
		}
	}
}

func (p *TracePrinter) printTraceHTML(tsFrom, tsTo *TraceStats) {
	var data [][]string

	if tsTo == nil {
		for _, s := range tsFrom.stats {
			data = append(data, p.GenerateTraceLine(s, true))
		}
	} else {
		for _, to := range tsTo.stats {
			from := findTraceStatFrom(tsFrom, to)

			if from == nil {
				data = append(data, p.GenerateTraceLine(to, false))
			} else {
				data = append(data, p.GenerateTraceLineWithDiff(from, to, false))
			}
		}
	}
	content, _ := html.RenderTableWithGridJS("alp", p.headers, data, p.printOptions.paginationLimit)
	fmt.Println(content)
}
