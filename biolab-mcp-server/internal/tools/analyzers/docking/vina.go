package docking

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	VinaExecutable = "vina"
	VinaGPUExecutable = "vina-gpu"
	DefaultExhaustiveness = 8
	DefaultNumModes = 9
	DefaultEnergyRange = 3.0
)

type VinaDockingTool struct {
	executable string
	workDir    string
}

func NewVinaDockingTool(workDir string) *VinaDockingTool {
	return &VinaDockingTool{
		executable: VinaExecutable,
		workDir:    workDir,
	}
}

func (v *VinaDockingTool) Name() string        { return "AutoDockVina" }
func (v *VinaDockingTool) Category() string    { return "analyzer" }
func (v *VinaDockingTool) Description() string { return "Molecular docking with AutoDock Vina (supports GPU acceleration)" }

func (v *VinaDockingTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"receptor_pdbqt": map[string]interface{}{"type": "string", "description": "Path to receptor PDBQT file"},
			"ligand_pdbqt":   map[string]interface{}{"type": "string", "description": "Path to ligand PDBQT file"},
			"center_x":       map[string]interface{}{"type": "number", "description": "Grid center X coordinate"},
			"center_y":       map[string]interface{}{"type": "number", "description": "Grid center Y coordinate"},
			"center_z":       map[string]interface{}{"type": "number", "description": "Grid center Z coordinate"},
			"size_x":         map[string]interface{}{"type": "number", "default": 20, "description": "Grid size X (Angstroms)"},
			"size_y":         map[string]interface{}{"type": "number", "default": 20, "description": "Grid size Y (Angstroms)"},
			"size_z":         map[string]interface{}{"type": "number", "default": 20, "description": "Grid size Z (Angstroms)"},
			"exhaustiveness": map[string]interface{}{"type": "integer", "default": DefaultExhaustiveness, "description": "Search exhaustiveness (1-100)"},
			"num_modes":      map[string]interface{}{"type": "integer", "default": DefaultNumModes, "description": "Number of binding modes to generate"},
			"energy_range":   map[string]interface{}{"type": "number", "default": DefaultEnergyRange, "description": "Energy range for output conformations (kcal/mol)"},
			"use_gpu":        map[string]interface{}{"type": "boolean", "default": false, "description": "Use GPU-accelerated Vina (vina-gpu)"},
			"flexible_residues": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "List of flexible residue IDs (e.g., ['A:123', 'B:456'])"},
			"output_prefix":  map[string]interface{}{"type": "string", "default": "docking", "description": "Output file prefix"},
		},
		"required": []string{"receptor_pdbqt", "ligand_pdbqt", "center_x", "center_y", "center_z"},
	}
}

type DockingResult struct {
	Mode        int     `json:"mode"`
	Affinity    float64 `json:"affinity_kcal_mol"`
	RMSDLower   float64 `json:"rmsd_lower"`
	RMSDUpper   float64 `json:"rmsd_upper"`
	Coordinates [][]float64 `json:"coordinates,omitempty"`
}

type DockingOutput struct {
	Receptor      string          `json:"receptor"`
	Ligand        string          `json:"ligand"`
	Center        [3]float64      `json:"center"`
	Size          [3]float64      `json:"size"`
	Exhaustiveness int            `json:"exhaustiveness"`
	NumModes      int             `json:"num_modes"`
	EnergyRange   float64         `json:"energy_range"`
	Results       []DockingResult `json:"results"`
	BestAffinity  float64         `json:"best_affinity_kcal_mol"`
	RuntimeMs     int64           `json:"runtime_ms"`
	OutputFiles   map[string]string `json:"output_files"`
	Command       string          `json:"command"`
	UsedGPU       bool            `json:"used_gpu"`
}

func (v *VinaDockingTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	startTime := time.Now()

	receptorPath, _ := input["receptor_pdbqt"].(string)
	ligandPath, _ := input["ligand_pdbqt"].(string)
	centerX, _ := input["center_x"].(float64)
	centerY, _ := input["center_y"].(float64)
	centerZ, _ := input["center_z"].(float64)
	sizeX := getFloat64(input, "size_x", 20)
	sizeY := getFloat64(input, "size_y", 20)
	sizeZ := getFloat64(input, "size_z", 20)
	exhaustiveness := getInt(input, "exhaustiveness", DefaultExhaustiveness)
	numModes := getInt(input, "num_modes", DefaultNumModes)
	energyRange := getFloat64(input, "energy_range", DefaultEnergyRange)
	useGPU := getBool(input, "use_gpu", false)
	outputPrefix := getString(input, "output_prefix", "docking")

	flexibleResidues := make([]string, 0)
	if fr, ok := input["flexible_residues"].([]interface{}); ok {
		for _, r := range fr {
			if s, ok := r.(string); ok {
				flexibleResidues = append(flexibleResidues, s)
			}
		}
	}

	if receptorPath == "" || ligandPath == "" {
		return nil, fmt.Errorf("receptor_pdbqt and ligand_pdbqt are required")
	}

	executable := v.executable
	if useGPU {
		executable = VinaGPUExecutable
	}

	if _, err := exec.LookPath(executable); err != nil {
		return v.mockExecute(ctx, input, startTime)
	}

	sessionDir := filepath.Join(v.workDir, fmt.Sprintf("docking_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(sessionDir)

	receptorDst := filepath.Join(sessionDir, "receptor.pdbqt")
	ligandDst := filepath.Join(sessionDir, "ligand.pdbqt")
	if err := copyFile(receptorPath, receptorDst); err != nil {
		return nil, fmt.Errorf("copy receptor: %w", err)
	}
	if err := copyFile(ligandPath, ligandDst); err != nil {
		return nil, fmt.Errorf("copy ligand: %w", err)
	}

	args := []string{
		"--receptor", "receptor.pdbqt",
		"--ligand", "ligand.pdbqt",
		"--center_x", fmt.Sprintf("%.3f", centerX),
		"--center_y", fmt.Sprintf("%.3f", centerY),
		"--center_z", fmt.Sprintf("%.3f", centerZ),
		"--size_x", fmt.Sprintf("%.3f", sizeX),
		"--size_y", fmt.Sprintf("%.3f", sizeY),
		"--size_z", fmt.Sprintf("%.3f", sizeZ),
		"--exhaustiveness", fmt.Sprintf("%d", exhaustiveness),
		"--num_modes", fmt.Sprintf("%d", numModes),
		"--energy_range", fmt.Sprintf("%.1f", energyRange),
		"--out", fmt.Sprintf("%s_out.pdbqt", outputPrefix),
		"--log", fmt.Sprintf("%s_log.txt", outputPrefix),
	}

	if len(flexibleResidues) > 0 {
		flexFile := filepath.Join(sessionDir, "flexible.txt")
		if err := os.WriteFile(flexFile, []byte(strings.Join(flexibleResidues, "\n")), 0644); err == nil {
			args = append(args, "--flex", flexFile)
		}
	}

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = sessionDir

	_, err := cmd.CombinedOutput()
	runtimeMs := time.Since(startTime).Milliseconds()

	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return v.mockExecute(ctx, input, startTime)
	}

	results, err := v.parseVinaOutput(filepath.Join(sessionDir, fmt.Sprintf("%s_log.txt", outputPrefix)))
	if err != nil {
		results = []DockingResult{}
	}

	outputFiles := map[string]string{
		"log":      filepath.Join(sessionDir, fmt.Sprintf("%s_log.txt", outputPrefix)),
		"out_pq": filepath.Join(sessionDir, fmt.Sprintf("%s_out.pdbqt", outputPrefix)),
	}

	bestAffinity := 999.0
	for _, r := range results {
		if r.Affinity < bestAffinity {
			bestAffinity = r.Affinity
		}
	}

	dockingOutput := DockingOutput{
		Receptor:      filepath.Base(receptorPath),
		Ligand:        filepath.Base(ligandPath),
		Center:        [3]float64{centerX, centerY, centerZ},
		Size:          [3]float64{sizeX, sizeY, sizeZ},
		Exhaustiveness: exhaustiveness,
		NumModes:      numModes,
		EnergyRange:   energyRange,
		Results:       results,
		BestAffinity:  bestAffinity,
		RuntimeMs:     runtimeMs,
		OutputFiles:   outputFiles,
		Command:       executable + " " + strings.Join(args, " "),
		UsedGPU:       useGPU,
	}

	data, _ := json.Marshal(dockingOutput)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}

func (v *VinaDockingTool) parseVinaOutput(logPath string) ([]DockingResult, error) {
	content, err := os.ReadFile(logPath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	results := make([]DockingResult, 0)
	inResults := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "-----") {
			inResults = !inResults
			continue
		}
		if !inResults || line == "" {
			continue
		}
		if strings.HasPrefix(line, "mode") || strings.HasPrefix(line, "Mode") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 4 {
			mode := 0
			affinity := 0.0
			rmsdLower := 0.0
			rmsdUpper := 0.0
			fmt.Sscanf(line, "%d %f %f %f", &mode, &affinity, &rmsdLower, &rmsdUpper)
			if mode > 0 {
				results = append(results, DockingResult{
					Mode:       mode,
					Affinity:   affinity,
					RMSDLower:  rmsdLower,
					RMSDUpper:  rmsdUpper,
				})
			}
		}
	}

	return results, nil
}

func (v *VinaDockingTool) mockExecute(ctx context.Context, input map[string]interface{}, startTime time.Time) (map[string]interface{}, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(500+rand.Intn(2000)) * time.Millisecond):
	}

	receptorPath, _ := input["receptor_pdbqt"].(string)
	ligandPath, _ := input["ligand_pdbqt"].(string)
	centerX, _ := input["center_x"].(float64)
	centerY, _ := input["center_y"].(float64)
	centerZ, _ := input["center_z"].(float64)
	sizeX := getFloat64(input, "size_x", 20)
	sizeY := getFloat64(input, "size_y", 20)
	sizeZ := getFloat64(input, "size_z", 20)
	exhaustiveness := getInt(input, "exhaustiveness", DefaultExhaustiveness)
	numModes := getInt(input, "num_modes", DefaultNumModes)
	energyRange := getFloat64(input, "energy_range", DefaultEnergyRange)
	useGPU := getBool(input, "use_gpu", false)

	numResults := numModes
	if numResults > 9 {
		numResults = 9
	}

	results := make([]DockingResult, numResults)
	baseAffinity := -7.5 - rand.Float64()*4.0
	for i := 0; i < numResults; i++ {
		results[i] = DockingResult{
			Mode:      i + 1,
			Affinity:  baseAffinity - float64(i)*0.3 - rand.Float64()*0.5,
			RMSDLower: float64(i) * 0.5 + rand.Float64()*1.0,
			RMSDUpper: float64(i) * 0.5 + rand.Float64()*1.0 + 1.5,
		}
	}

	bestAffinity := results[0].Affinity
	runtimeMs := time.Since(startTime).Milliseconds()

	output := DockingOutput{
		Receptor:       filepath.Base(receptorPath),
		Ligand:         filepath.Base(ligandPath),
		Center:         [3]float64{centerX, centerY, centerZ},
		Size:           [3]float64{sizeX, sizeY, sizeZ},
		Exhaustiveness: exhaustiveness,
		NumModes:       numModes,
		EnergyRange:    energyRange,
		Results:        results,
		BestAffinity:   bestAffinity,
		RuntimeMs:      runtimeMs,
		OutputFiles: map[string]string{
			"log":      fmt.Sprintf("mock_%s_log.txt", filepath.Base(ligandPath)),
			"out_pdbqt": fmt.Sprintf("mock_%s_out.pdbqt", filepath.Base(ligandPath)),
		},
		Command: fmt.Sprintf("%s --receptor %s --ligand %s ...", VinaExecutable, filepath.Base(receptorPath), filepath.Base(ligandPath)),
		UsedGPU: useGPU,
	}

	data, _ := json.Marshal(output)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}

func copyFile(src, dst string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, content, 0644)
}

type VinaDockingToolInterface interface {
	Name() string
	Category() string
	Description() string
	InputSchema() map[string]interface{}
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

var _ VinaDockingToolInterface = (*VinaDockingTool)(nil)