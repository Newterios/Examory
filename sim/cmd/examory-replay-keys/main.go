// Command examory-replay-keys recreates the simulator's fixed quiz roster for API replay.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Newterios/Examory/sim/internal/params"
	"github.com/Newterios/Examory/sim/internal/world"
)

type fileRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type manifest struct {
	Simulator string `json:"simulator"`
	Args      struct {
		ItemSampling string `json:"item_sampling"`
		Questions    int    `json:"questions"`
		Seed         int64  `json:"seed"`
	} `json:"args"`
	Params map[string]fileRef `json:"params"`
}

func export(manifestPath, modelPath, mechanismPath, output string) error {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	if m.Simulator != "examory-sim" || m.Args.ItemSampling != world.SamplingFixed {
		return errors.New("expected an examory-sim manifest with fixed item sampling")
	}
	if modelPath == "" {
		modelPath = m.Params["params"].Path
	}
	if mechanismPath == "" {
		mechanismPath = m.Params["mechanisms"].Path
	}
	model, modelSum, err := params.LoadModel(modelPath)
	if err != nil {
		return err
	}
	mechanisms, mechanismSum, err := params.LoadMechanisms(mechanismPath)
	if err != nil {
		return err
	}
	if modelSum != m.Params["params"].SHA256 || mechanismSum != m.Params["mechanisms"].SHA256 {
		return errors.New("simulator parameter checksum differs from the manifest")
	}
	// Every attempt receives this same roster. A single honest sitting suffices for any world size.
	cfg := &world.Config{Students: 1, Attempts: 1, Questions: m.Args.Questions, Seed: m.Args.Seed,
		Model: model, Mech: mechanisms, StartAt: time.Unix(0, 0).UTC(), Worlds: []string{world.Honest},
		ItemSampling: world.SamplingFixed, Weights: [4]float64{1, 0, 0, 0}}
	result, err := world.Generate(cfg)
	if err != nil {
		return err
	}
	keys := make(map[string]int, m.Args.Questions)
	for _, item := range result.Worlds[world.Honest][0].Attempt.Items {
		keys[item.ID] = item.Key
	}
	encoded, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(output, append(encoded, '\n'), 0600)
}

func main() {
	input := flag.String("manifest", "", "examory-sim generate manifest.json")
	model := flag.String("params", "", "model parameter path (default: path in manifest)")
	mechanisms := flag.String("mechanisms", "", "mechanism parameter path (default: path in manifest)")
	output := flag.String("out", "", "destination JSON object of synthetic answer keys")
	flag.Parse()
	if *input == "" || *output == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := export(*input, *model, *mechanisms, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
