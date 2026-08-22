package kb

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestEmbeddedMetadataMatchesRustToolchainLock(t *testing.T) {
	b, err := os.ReadFile("../../deploy/rust-toolchain.toml")
	if err != nil {
		t.Fatalf("read rust-toolchain lock: %v", err)
	}
	match := regexp.MustCompile(`(?m)^channel\s*=\s*"([^"]+)"`).FindStringSubmatch(string(b))
	if len(match) != 2 {
		t.Fatalf("toolchain channel missing: %s", b)
	}
	if got, want := EmbeddedRustcVersion(), match[1]; got != want {
		t.Fatalf("embedded KB rustc version = %q, deploy lock = %q; re-import/update KB metadata together", got, want)
	}
}

func TestBuildBlobCarriesEmbeddedMetadata(t *testing.T) {
	b, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	atoms, meta, err := decodeBlob(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(atoms) != 683 {
		t.Fatalf("atom count = %d", len(atoms))
	}
	if meta != EmbeddedMetadata() {
		t.Fatalf("blob metadata = %+v, embedded = %+v", meta, EmbeddedMetadata())
	}
}

func TestLegacyV1BlobRemainsReadableButHasNoVersionProvenance(t *testing.T) {
	atoms, _, err := buildAtoms()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy-kb.ykc")
	if err := os.WriteFile(path, encodeLegacyV1ForTest(t, atoms), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile legacy v1: %v", err)
	}
	if st.Count() != len(atoms) || st.RustcVersion() != "" || st.ErrorIndexURL() != "" {
		t.Fatalf("legacy metadata should be unknown: count=%d meta=%+v", st.Count(), st.Metadata())
	}
}

func encodeLegacyV1ForTest(t *testing.T, atoms []*Atom) []byte {
	t.Helper()
	var payload []byte
	for _, atom := range atoms {
		j, err := json.Marshal(atom)
		if err != nil {
			t.Fatal(err)
		}
		var lb [4]byte
		binary.BigEndian.PutUint32(lb[:], uint32(len(j)))
		payload = append(payload, lb[:]...)
		payload = append(payload, j...)
	}
	out := append([]byte(blobMagic), make([]byte, 4+8)...)
	binary.BigEndian.PutUint32(out[6:10], blobVersionV1)
	binary.BigEndian.PutUint64(out[10:18], uint64(len(atoms)))
	out = append(out, payload...)
	sum := sha256.Sum256(payload)
	return append(out, sum[:]...)
}
