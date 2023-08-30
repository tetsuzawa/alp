package stats

import (
	"io"

	"gopkg.in/yaml.v2"
)

func (ts *TraceStats) DumpStats(w io.Writer) error {
	buf, err := yaml.Marshal(&ts.ScenarioStats)
	if err != nil {
		return err
	}

	_, err = w.Write(buf)

	return err
}
