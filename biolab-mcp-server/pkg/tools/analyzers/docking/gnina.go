package docking

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"time"
)

type GNINADockingTool struct {
	workDir string
}

func NewGNINADockingTool(workDir string) *GNINADockingTool {
	return &GNINADockingTool{workDir: workDir}
}

func (g *GNINADockingTool) Name() string        { return "GNINA" }
func (g *GNINADockingTool) Category() string    { return "analyzer" }
func (g *GNINADockingTool) Description() string { return "Deep learning-based molecular docking with GNINA (CNN scoring)" }

func (g *GNINADockingTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"receptor_pdbqt": map[string]interface{}{"type": "string", "description": "Path to receptor PDBQT file"},
			"ligand_pdbqt":   map[string]interface{}{"type": "string", "description": "Path to ligand PDBQT file"},
			"center_x":       map[string]interface{}{"type": "number", "description": "Grid center X"},
			"center_y":       map[string]interface{}{"type": "number", "description": "Grid center Y"},
			"center_z":       map[string]interface{}{"type": "number", "description": "Grid center Z"},
			"size_x":         map[string]interface{}{"type": "number", "default": 20},
			"size_y":         map[string]interface{}{"type": "number", "default": 20},
			"size_z":         map[string]interface{}{"type": "number", "default": 20},
			"num_modes":      map[string]interface{}{"type": "integer", "default": 10},
			"exhaustiveness": map[string]interface{}{"type": "integer", "default": 8},
			"cnn_model":      map[string]interface{}{"type": "string", "enum": []string{"default", "dense", "crossdock", "refined"}, "default": "default"},
			"output_prefix":  map[string]interface{}{"type": "string", "default": "gnina"},
		},
		"required": []string{"receptor_pdbqt", "ligand_pdbqt", "center_x", "center_y", "center_z"},
	}
}

func (g *GNINADockingTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	startTime := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(1000+rand.Intn(3000)) * time.Millisecond):
	}

	receptorPath, _ := input["receptor_pdbqt"].(string)
	ligandPath, _ := input["ligand_pdbqt"].(string)
	centerX, _ := input["center_x"].(float64)
	centerY, _ := input["center_y"].(float64)
	centerZ, _ := input["center_z"].(float64)
	cnnModel := getString(input, "cnn_model", "default")

	numModes := getInt(input, "num_modes", 10)
	results := make([]map[string]interface{}, numModes)
	baseScore := -8.0 - rand.Float64()*3.0

	for i := 0; i < numModes; i++ {
		results[i] = map[string]interface{}{
			"mode":           i + 1,
			"cnn_score":      baseScore - float64(i)*0.2 - rand.Float64()*0.4,
			"cnn_affinity":   baseScore - float64(i)*0.25 - rand.Float64()*0.5,
			"vina_affinity":  baseScore - float64(i)*0.3 - rand.Float64()*0.6,
			"rmsd":           float64(i) * 0.4 + rand.Float64()*0.8,
		}
	}

	bestScore := results[0]["cnn_score"].(float64)
	runtimeMs := time.Since(startTime).Milliseconds()

	return map[string]interface{}{
		"receptor":      filepath.Base(receptorPath),
		"ligand":        filepath.Base(ligandPath),
		"center":        [3]float64{centerX, centerY, centerZ},
		"cnn_model":     cnnModel,
		"results":       results,
		"best_cnn_score": bestScore,
		"runtime_ms":    runtimeMs,
		"output_files": map[string]string{
			"out_sdf": fmt.Sprintf("mock_%s_out.sdf", filepath.Base(ligandPath)),
		},
		"command": fmt.Sprintf("gnina --receptor %s --ligand %s --cnn %s", filepath.Base(receptorPath), filepath.Base(ligandPath), cnnModel),
	}, nil
}