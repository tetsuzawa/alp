package stats

import "fmt"

type TraceDiffer struct {
	From *TraceStat
	To   *TraceStat
}

func NewTraceDiffer(from, to *TraceStat) *TraceDiffer {
	return &TraceDiffer{
		From: from,
		To:   to,
	}
}

func (d *TraceDiffer) DiffCnt() string {
	v := d.To.Cnt - d.From.Cnt
	if v >= 0 {
		return fmt.Sprintf("+%d", v)
	}

	return fmt.Sprintf("%d", v)
}

func (d *TraceDiffer) DiffMaxResponseTime() string {
	v := d.To.MaxResponseTime() - d.From.MaxResponseTime()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffMinResponseTime() string {
	v := d.To.MinResponseTime() - d.From.MinResponseTime()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffSumResponseTime() string {
	v := d.To.SumResponseTime() - d.From.SumResponseTime()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffAvgResponseTime() string {
	v := d.To.AvgResponseTime() - d.From.AvgResponseTime()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffPNResponseTime(n int) string {
	v := d.To.PNResponseTime(n) - d.From.PNResponseTime(n)
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffStddevResponseTime() string {
	v := d.To.StddevResponseTime() - d.From.StddevResponseTime()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

// request
func (d *TraceDiffer) DiffMaxRequestBodyBytes() string {
	v := d.To.MaxRequestBodyBytes() - d.From.MaxRequestBodyBytes()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffMinRequestBodyBytes() string {
	v := d.To.MinRequestBodyBytes() - d.From.MinRequestBodyBytes()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffSumRequestBodyBytes() string {
	v := d.To.SumRequestBodyBytes() - d.From.SumRequestBodyBytes()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffAvgRequestBodyBytes() string {
	v := d.To.AvgRequestBodyBytes() - d.From.AvgRequestBodyBytes()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffPNRequestBodyBytes(n int) string {
	v := d.To.PNRequestBodyBytes(n) - d.From.PNRequestBodyBytes(n)
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffStddevRequestBodyBytes() string {
	v := d.To.StddevRequestBodyBytes() - d.From.StddevRequestBodyBytes()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

// response
func (d *TraceDiffer) DiffMaxResponseBodyBytes() string {
	v := d.To.MaxResponseBodyBytes() - d.From.MaxResponseBodyBytes()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffMinResponseBodyBytes() string {
	v := d.To.MinResponseBodyBytes() - d.From.MinResponseBodyBytes()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffSumResponseBodyBytes() string {
	v := d.To.SumResponseBodyBytes() - d.From.SumResponseBodyBytes()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffAvgResponseBodyBytes() string {
	v := d.To.AvgResponseBodyBytes() - d.From.AvgResponseBodyBytes()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffPNResponseBodyBytes(n int) string {
	v := d.To.PNResponseBodyBytes(n) - d.From.PNResponseBodyBytes(n)
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func (d *TraceDiffer) DiffStddevResponseBodyBytes() string {
	v := d.To.StddevResponseBodyBytes() - d.From.StddevResponseBodyBytes()
	if v >= 0 {
		return fmt.Sprintf("+%.3f", v)
	}

	return fmt.Sprintf("%.3f", v)
}

func TraceDiffCountAll(from, to map[string]int) map[string]string {
	counts := make(map[string]string, 6)
	keys := []string{"count"}

	for _, key := range keys {
		v := to[key] - from[key]
		if v >= 0 {
			counts[key] = fmt.Sprintf("+%d", v)
		} else {
			counts[key] = fmt.Sprintf("-%d", v)
		}
	}

	return counts
}
