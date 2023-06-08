package format

import (
	"bytes"
	"crypto/sha256"
	"encoding"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type UUID = uuid.UUID

type Hash struct {
	Algorithm, Digest string
}

func SHA256(b []byte) Hash {
	digest := sha256.Sum256(b)
	return Hash{
		Algorithm: "sha256",
		Digest:    hex.EncodeToString(digest[:]),
	}
}

func ParseHash(s string) Hash {
	algorithm, digest, ok := strings.Cut(s, ":")
	if !ok {
		algorithm, digest = "sha256", s
	}
	return Hash{Algorithm: algorithm, Digest: digest}
}

func (h Hash) Short() string {
	s := h.Digest
	if len(s) > 12 {
		s = s[:12]
	}
	return s
}

func (h Hash) String() string {
	return h.Algorithm + ":" + h.Digest
}

func (h Hash) MarshalText() ([]byte, error) {
	return []byte(h.String()), nil
}

func (h *Hash) UnmarshalText(b []byte) error {
	algorithm, digest, ok := strings.Cut(string(b), ":")
	if !ok {
		return fmt.Errorf("malformed hash: %q", b)
	}
	h.Algorithm, h.Digest = algorithm, digest
	return nil
}

var (
	_ encoding.TextMarshaler   = Hash{}
	_ encoding.TextUnmarshaler = (*Hash)(nil)
)

type MediaType string

const (
	TypeDescriptor        MediaType = "application/vnd.oci.descriptor.v1+json"
	TypeTimecraftRuntime  MediaType = "application/vnd.timecraft.runtime.v1+json"
	TypeTimecraftConfig   MediaType = "application/vnd.timecraft.config.v1+json"
	TypeTimecraftProcess  MediaType = "application/vnd.timecraft.process.v1+json"
	TypeTimecraftProfile  MediaType = "application/vnd.timecraft.profile.v1+pprof"
	TypeTimecraftManifest MediaType = "application/vnd.timecraft.manifest.v1+json"
	TypeTimecraftModule   MediaType = "application/vnd.timecraft.module.v1+wasm"
)

func (m MediaType) String() string { return string(m) }

type Resource interface {
	ContentType() MediaType
}

type ResourceMarshaler interface {
	Resource
	MarshalResource() ([]byte, error)
}

type ResourceUnmarshaler interface {
	Resource
	UnmarshalResource([]byte) error
}

type Descriptor struct {
	MediaType   MediaType         `json:"mediaType"             yaml:"mediaType"`
	Digest      Hash              `json:"digest"                yaml:"digest"`
	Size        int64             `json:"size"                  yaml:"size"`
	URLs        []string          `json:"urls,omitempty"        yaml:"urls,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

func (d *Descriptor) ContentType() MediaType {
	return TypeDescriptor
}

func (d *Descriptor) MarshalResource() ([]byte, error) {
	return jsonEncode(d)
}

func (d *Descriptor) UnmarshalResource(b []byte) error {
	return jsonDecode(b, d)
}

type Module struct {
	Code []byte
}

func (m *Module) ContentType() MediaType {
	return TypeTimecraftModule
}

func (m *Module) MarshalResource() ([]byte, error) {
	return m.Code, nil
}

func (m *Module) UnmarshalResource(b []byte) error {
	m.Code = b
	return nil
}

type Runtime struct {
	Runtime string `json:"runtime" yaml:"runtime"`
	Version string `json:"version" yaml:"version"`
}

func (r *Runtime) ContentType() MediaType {
	return TypeTimecraftRuntime
}

func (r *Runtime) MarshalResource() ([]byte, error) {
	return jsonEncode(r)
}

func (r *Runtime) UnmarshalResource(b []byte) error {
	return jsonDecode(b, r)
}

type Config struct {
	Runtime *Descriptor   `json:"runtime"       yaml:"runtime"`
	Modules []*Descriptor `json:"modules"       yaml:"modules"`
	Args    []string      `json:"args"          yaml:"args"`
	Env     []string      `json:"env,omitempty" yaml:"env,omitempty"`
}

func (c *Config) ContentType() MediaType {
	return TypeTimecraftConfig
}

func (c *Config) MarshalResource() ([]byte, error) {
	return jsonEncode(c)
}

func (c *Config) UnmarshalResource(b []byte) error {
	return jsonDecode(b, c)
}

type Process struct {
	ID        UUID        `json:"id"        yaml:"id"`
	StartTime time.Time   `json:"startTime" yaml:"startTime"`
	Config    *Descriptor `json:"config"    yaml:"config"`
}

func (p *Process) ContentType() MediaType {
	return TypeTimecraftProcess
}

func (p *Process) MarshalResource() ([]byte, error) {
	return jsonEncode(p)
}

func (p *Process) UnmarshalResource(b []byte) error {
	return jsonDecode(b, p)
}

func jsonEncode(value any) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(value)
	return buf.Bytes(), err
}

func jsonDecode(b []byte, value any) error {
	return json.Unmarshal(b, &value)
}

type Manifest struct {
	ProcessID UUID         `json:"-"                  yaml:"-"`
	StartTime time.Time    `json:"startTime"          yaml:"startTime"`
	Process   *Descriptor  `json:"process"            yaml:"process"`
	Segments  []LogSegment `json:"segments,omitempty" yaml:"segments,omitempty"`
}

func (m *Manifest) ContentType() MediaType {
	return TypeTimecraftManifest
}

func (m *Manifest) MarshalResource() ([]byte, error) {
	return jsonEncode(m)
}

func (m *Manifest) UnmarshalResource(b []byte) error {
	return jsonDecode(b, m)
}

type LogSegment struct {
	Number    int       `json:"number"    yaml:"number"`
	Size      int64     `json:"size"      yaml:"size"`
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`
}

type Record struct {
	ID       string
	Process  *Descriptor
	Offset   int64
	Size     int64
	Time     time.Time
	Function string
}

var (
	_ ResourceMarshaler = (*Descriptor)(nil)
	_ ResourceMarshaler = (*Module)(nil)
	_ ResourceMarshaler = (*Runtime)(nil)
	_ ResourceMarshaler = (*Config)(nil)
	_ ResourceMarshaler = (*Manifest)(nil)

	_ ResourceUnmarshaler = (*Descriptor)(nil)
	_ ResourceUnmarshaler = (*Module)(nil)
	_ ResourceUnmarshaler = (*Runtime)(nil)
	_ ResourceUnmarshaler = (*Config)(nil)
	_ ResourceUnmarshaler = (*Manifest)(nil)
)
