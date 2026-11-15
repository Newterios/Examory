package events

import (
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"testing"

	"github.com/Newterios/Examory/sim/internal/world"
)

// SHA-256 of the seed-42 default outputs of simulator version 0.1.0 (before
// copy_peer.sync_submit_* was added), produced on darwin/arm64. With
// sync_submit_prob = 0 the current simulator must reproduce them exactly:
// the sync decision uses its own stream and shifts no other draw.
var golden010 = map[string]string{
	world.Honest: "1e705c2f612a3aa8085af10093ad057454b41657fa05729706507fee9caf5b15",
	world.FI:     "ebf392137b278ff8e7754a5d6e6bbeec6d7bed625d42ef7d66653563de49a074",
	world.BL:     "23a96786c997d7c7e2b4e7791c6a991a77a007d91caf46c29e45774a6fdf4482",
}

func TestSyncOffReproducesVersion010(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("golden hashes were produced on arm64; float rounding may differ elsewhere")
	}
	if testing.Short() {
		t.Skip("full 2000-student run")
	}
	cfg := config(t, 2000, world.SamplingFixed)
	cfg.Mech.CopyPeer.SyncSubmitProb = 0
	res, err := world.Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range world.AllWorlds {
		h := sha256.Sum256(csvBytes(t, cfg.Seed, res.Worlds[w]))
		if got := hex.EncodeToString(h[:]); got != golden010[w] {
			t.Fatalf("%s.csv: %s, want %s", w, got, golden010[w])
		}
	}
}
