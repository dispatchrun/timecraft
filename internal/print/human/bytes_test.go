package human

import (
	"encoding/json"
	"fmt"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestBytesParse(t *testing.T) {
	for _, test := range []struct {
		in  string
		out Bytes
	}{
		{in: "0", out: 0},

		{in: "2B", out: 2},
		{in: "2K", out: 2 * KB},
		{in: "2M", out: 2 * MB},
		{in: "2G", out: 2 * GB},
		{in: "2T", out: 2 * TB},
		{in: "2P", out: 2 * PB},

		{in: "2", out: 2},
		{in: "2 KiB", out: 2 * KiB},
		{in: "2 MiB", out: 2 * MiB},
		{in: "2 GiB", out: 2 * GiB},
		{in: "2 TiB", out: 2 * TiB},
		{in: "2 PiB", out: 2 * PiB},

		{in: "1.234 K", out: 1234},
		{in: "1.234 M", out: 1234 * KB},

		{in: "1.5 Ki", out: 1*KiB + 512},
		{in: "1.5 Mi", out: 1*MiB + 512*KiB},
	} {
		t.Run(test.in, func(t *testing.T) {
			b, err := ParseBytes(test.in)
			if err != nil {
				t.Fatal(err)
			}
			if b != test.out {
				t.Error("parsed bytes mismatch:", b, "!=", test.out)
			}
		})
	}
}

func TestBytesFormat(t *testing.T) {
	for _, test := range []struct {
		in  Bytes
		fmt string
		out string
	}{
		{fmt: "%v", out: "0", in: 0},
		{fmt: "%v", out: "2", in: 2},

		{fmt: "%v", out: "1.95 KiB", in: 2 * KB},
		{fmt: "%v", out: "1.91 MiB", in: 2 * MB},
		{fmt: "%v", out: "1.86 GiB", in: 2 * GB},
		{fmt: "%v", out: "1.82 TiB", in: 2 * TB},
		{fmt: "%v", out: "1.78 PiB", in: 2 * PB},

		{fmt: "%v", out: "2 KiB", in: 2 * KiB},
		{fmt: "%v", out: "2 MiB", in: 2 * MiB},
		{fmt: "%v", out: "2 GiB", in: 2 * GiB},
		{fmt: "%v", out: "2 TiB", in: 2 * TiB},
		{fmt: "%v", out: "2 PiB", in: 2 * PiB},

		{fmt: "%v", out: "1.21 KiB", in: 1234},
		{fmt: "%v", out: "1.18 MiB", in: 1234 * KB},

		{fmt: "%v", out: "1.5 KiB", in: 1*KiB + 512},
		{fmt: "%v", out: "1.5 MiB", in: 1*MiB + 512*KiB},

		{fmt: "%d", out: "123456789", in: 123456789},
		{fmt: "%b", out: "123 MB", in: 123456789},
		{fmt: "%s", out: "118 MiB", in: 123456789},
		{fmt: "%#v", out: "human.Bytes(123456789)", in: 123456789},
	} {
		t.Run(test.out, func(t *testing.T) {
			if s := fmt.Sprintf(test.fmt, test.in); s != test.out {
				t.Error("formatted bytes mismatch:", s, "!=", test.out)
			}
		})
	}
}

func TestBytesJSON(t *testing.T) {
	testBytesEncoding(t, 1*KiB, json.Marshal, json.Unmarshal)
}

func TestBytesYAML(t *testing.T) {
	testBytesEncoding(t, 1*KiB, yaml.Marshal, yaml.Unmarshal)
}

func testBytesEncoding(t *testing.T, x Bytes, marshal func(any) ([]byte, error), unmarshal func([]byte, any) error) {
	b, err := marshal(x)
	if err != nil {
		t.Fatal("marshal error:", err)
	}

	v := Bytes(0)
	if err := unmarshal(b, &v); err != nil {
		t.Error("unmarshal error:", err)
	} else if v != x {
		t.Error("value mismatch:", v, "!=", x)
	}
}
