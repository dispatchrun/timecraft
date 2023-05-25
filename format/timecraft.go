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
	TypeTimecraftManifest MediaType = "application/vnd.timecraft.manifest.v1+json"
	TypeTimecraftModule   MediaType = "application/vnd.timecraft.module.v1+wasm"
)

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
	MediaType   MediaType         `json:"mediaType"`
	Digest      Hash              `json:"digest"`
	Size        int64             `json:"size"`
	URLs        []string          `json:"urls,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
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

type Module []byte

func (m Module) ContentType() MediaType {
	return TypeTimecraftModule
}

func (m Module) MarshalResource() ([]byte, error) {
	return ([]byte)(m), nil
}

func (m *Module) UnmarshalResource(b []byte) error {
	*m = Module(b)
	return nil
}

type Runtime struct {
	Version string `json:"version"`
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
	Runtime *Descriptor   `json:"runtime"`
	Modules []*Descriptor `json:"modules"`
	Args    []string      `json:"args"`
	Env     []string      `json:"env,omitempty"`
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
	ID        UUID        `json:"id"`
	StartTime time.Time   `json:"startTime"`
	Config    *Descriptor `json:"config"`
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
	Process   *Descriptor `json:"process"`
	StartTime time.Time   `json:"startTime"`
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

var (
	_ ResourceMarshaler = (*Descriptor)(nil)
	_ ResourceMarshaler = (Module)(nil)
	_ ResourceMarshaler = (*Runtime)(nil)
	_ ResourceMarshaler = (*Config)(nil)
	_ ResourceMarshaler = (*Manifest)(nil)

	_ ResourceUnmarshaler = (*Descriptor)(nil)
	_ ResourceUnmarshaler = (*Module)(nil)
	_ ResourceUnmarshaler = (*Runtime)(nil)
	_ ResourceUnmarshaler = (*Config)(nil)
	_ ResourceUnmarshaler = (*Manifest)(nil)
)
