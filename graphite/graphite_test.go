package graphite_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/influxdb/influxdb"
	"github.com/influxdb/influxdb/graphite"
)

func Test_DecodeNameAndTags(t *testing.T) {
	var tests = []struct {
		test      string
		str       string
		name      string
		tags      map[string]string
		position  string
		separator string
		err       string
	}{
		{test: "metric only", str: "cpu", name: "cpu"},
		{test: "metric with single series", str: "cpu.hostname.server01", name: "cpu", tags: map[string]string{"hostname": "server01"}},
		{test: "metric with multiple series", str: "cpu.region.us-west.hostname.server01", name: "cpu", tags: map[string]string{"hostname": "server01", "region": "us-west"}},
		{test: "no metric", tags: make(map[string]string), err: `no name specified for metric. ""`},
		{test: "wrong metric format", str: "foo.cpu", tags: make(map[string]string), err: `received "foo.cpu" which doesn't conform to format of key.value.key.value.metric or metric`},
	}

	for _, test := range tests {
		t.Logf("testing %q...", test.test)

		p := graphite.NewParser()
		if test.separator != "" {
			p.Separator = test.separator
		}

		name, tags, err := p.DecodeNameAndTags(test.str)
		if errstr(err) != test.err {
			t.Fatalf("err does not match.  expected %v, got %v", test.err, err)
		}
		if name != test.name {
			t.Fatalf("name parse failer.  expected %v, got %v", test.name, name)
		}
		if len(tags) != len(test.tags) {
			t.Fatalf("unexpected number of tags.  expected %d, got %d", len(test.tags), len(tags))
		}
		for k, v := range test.tags {
			if tags[k] != v {
				t.Fatalf("unexpected tag value for tags[%s].  expected %q, got %q", k, v, tags[k])
			}
		}
	}
}

func Test_DecodeMetric(t *testing.T) {
	testTime := time.Now()
	epochTime := testTime.UnixNano() / 1000000 // nanos to milliseconds
	strTime := strconv.FormatInt(epochTime, 10)

	var tests = []struct {
		test                string
		line                string
		name                string
		tags                map[string]string
		isInt               bool
		iv                  int64
		fv                  float64
		timestamp           time.Time
		position, separator string
		err                 string
	}{
		{
			test:      "position first by default",
			line:      `cpu.foo.bar 50 ` + strTime + `\n`,
			name:      "cpu",
			tags:      map[string]string{"foo": "bar"},
			isInt:     true,
			iv:        50,
			timestamp: testTime,
		},
		{
			test:      "position first if unable to determine",
			position:  "foo",
			line:      `cpu.foo.bar 50 ` + strTime + `\n`,
			name:      "cpu",
			tags:      map[string]string{"foo": "bar"},
			isInt:     true,
			iv:        50,
			timestamp: testTime,
		},
		{
			test:      "position last if specified",
			position:  "last",
			line:      `foo.bar.cpu 50 ` + strTime + `\n`,
			name:      "cpu",
			tags:      map[string]string{"foo": "bar"},
			isInt:     true,
			iv:        50,
			timestamp: testTime,
		},
		{
			test:      "position first if specified with no series",
			position:  "first",
			line:      `cpu 50 ` + strTime + `\n`,
			name:      "cpu",
			tags:      map[string]string{},
			isInt:     true,
			iv:        50,
			timestamp: testTime,
		},
		{
			test:      "position last if specified with no series",
			position:  "last",
			line:      `cpu 50 ` + strTime + `\n`,
			name:      "cpu",
			tags:      map[string]string{},
			isInt:     true,
			iv:        50,
			timestamp: testTime,
		},
		{
			test:      "sepeartor is . by default",
			line:      `cpu.foo.bar 50 ` + strTime + `\n`,
			name:      "cpu",
			tags:      map[string]string{"foo": "bar"},
			isInt:     true,
			iv:        50,
			timestamp: testTime,
		},
		{
			test:      "sepeartor is . if specified",
			separator: ".",
			line:      `cpu.foo.bar 50 ` + strTime + `\n`,
			name:      "cpu",
			tags:      map[string]string{"foo": "bar"},
			isInt:     true,
			iv:        50,
			timestamp: testTime,
		},
		{
			test:      "sepeartor is - if specified",
			separator: "-",
			line:      `cpu-foo-bar 50 ` + strTime + `\n`,
			name:      "cpu",
			tags:      map[string]string{"foo": "bar"},
			isInt:     true,
			iv:        50,
			timestamp: testTime,
		},
		{
			test:      "sepeartor is boo if specified",
			separator: "boo",
			line:      `cpuboofooboobar 50 ` + strTime + `\n`,
			name:      "cpu",
			tags:      map[string]string{"foo": "bar"},
			isInt:     true,
			iv:        50,
			timestamp: testTime,
		},

		{
			test:      "series + metric + integer value",
			line:      `cpu.foo.bar 50 ` + strTime + `\n`,
			name:      "cpu",
			tags:      map[string]string{"foo": "bar"},
			isInt:     true,
			iv:        50,
			timestamp: testTime,
		},
		{
			test:      "metric only with float value",
			line:      `cpu 50.554 ` + strTime + `\n`,
			name:      "cpu",
			isInt:     false,
			fv:        50.554,
			timestamp: testTime,
		},
		{
			test: "missing metric",
			line: `50.554 1419972457825\n`,
			err:  `received "50.554 1419972457825" which doesn't have three fields`,
		},
		{
			test: "should fail on eof",
			line: ``,
			err:  `EOF`,
		},
		{
			test: "should fail on invalid key",
			line: `foo.cpu 50.554 1419972457825\n`,
			err:  `received "foo.cpu" which doesn't conform to format of key.value.key.value.metric or metric`,
		},
		{
			test: "should fail parsing invalid float",
			line: `cpu 50.554z 1419972457825\n`,
			err:  `strconv.ParseFloat: parsing "50.554z": invalid syntax`,
		},
		{
			test: "should fail parsing invalid int",
			line: `cpu 50z 1419972457825\n`,
			err:  `strconv.ParseFloat: parsing "50z": invalid syntax`,
		},
		{
			test: "should fail parsing invalid time",
			line: `cpu 50.554 14199724z57825\n`,
			err:  `strconv.ParseInt: parsing "14199724z57825": invalid syntax`,
		},
	}

	for _, test := range tests {
		t.Logf("testing %q...", test.test)

		p := graphite.NewParser()
		if test.separator != "" {
			p.Separator = test.separator
		}
		p.LastEnabled = (test.position == "last")

		m, err := p.Parse(test.line)
		if errstr(err) != test.err {
			t.Fatalf("err does not match.  expected %v, got %v", test.err, err)
		}
		if err != nil {
			// If we erred out,it was intended and the following tests won't work
			continue
		}
		if m.Name != test.name {
			t.Fatalf("name parse failer.  expected %v, got %v", test.name, m.Name)
		}
		if len(m.Tags) != len(test.tags) {
			t.Fatalf("tags len mismatch.  expected %d, got %d", len(test.tags), len(m.Tags))
		}
		if test.isInt {
			i := m.Value.(int64)
			if i != test.iv {
				t.Fatalf("integerValue value mismatch.  expected %v, got %v", test.iv, m.Value)
			}
		} else {
			f := m.Value.(float64)
			if m.Value != f {
				t.Fatalf("floatValue value mismatch.  expected %v, got %v", test.fv, f)
			}
		}
		if m.Timestamp.UnixNano()/1000000 != test.timestamp.UnixNano()/1000000 {
			t.Fatalf("timestamp value mismatch.  expected %v, got %v", test.timestamp.UnixNano(), m.Timestamp.UnixNano())
		}
	}
}

type testServer string

func (testServer) DefaultRetentionPolicy(database string) (*influxdb.RetentionPolicy, error) {
	return &influxdb.RetentionPolicy{}, nil
}
func (testServer) WriteSeries(database, retentionPolicy, name string, tags map[string]string, timestamp time.Time, values map[string]interface{}) error {
	return nil
}

type testServerFailedRetention string

func (testServerFailedRetention) DefaultRetentionPolicy(database string) (*influxdb.RetentionPolicy, error) {
	return &influxdb.RetentionPolicy{}, fmt.Errorf("no retention policy")
}
func (testServerFailedRetention) WriteSeries(database, retentionPolicy, name string, tags map[string]string, timestamp time.Time, values map[string]interface{}) error {
	return nil
}

type testServerFailedWrite string

func (testServerFailedWrite) DefaultRetentionPolicy(database string) (*influxdb.RetentionPolicy, error) {
	return &influxdb.RetentionPolicy{}, nil
}
func (testServerFailedWrite) WriteSeries(database, retentionPolicy, name string, tags map[string]string, timestamp time.Time, values map[string]interface{}) error {
	return fmt.Errorf("failed write")
}

// Test Helpers
func errstr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
