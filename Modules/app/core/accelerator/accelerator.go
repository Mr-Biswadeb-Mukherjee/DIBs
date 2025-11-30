//go:build ignore
// +build ignore

package accelerator

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// DeviceType represents the type of compute device.
type DeviceType string

const (
	CPU DeviceType = "cpu"
	GPU DeviceType = "gpu"
	TPU DeviceType = "tpu"
)

// Device holds information about a compute device.
type Device struct {
	Type  DeviceType
	ID    int
	Name  string
	Score int
	Cores int
}

// ResourceInfo holds the results of detection, including any messages.
type ResourceInfo struct {
	Devices  []Device
	Warnings []string
	Info     []string
	Errors   []error
}

// DetectDevices detects CPU, GPU, TPU in a vendor-agnostic way.
func DetectDevices() ResourceInfo {
	info := ResourceInfo{}
	numCPU := runtime.NumCPU()

	if numCPU < 1 {
		info.Errors = append(info.Errors, fmt.Errorf("no CPU cores detected"))
		return info
	}

	info.Info = append(info.Info, fmt.Sprintf("%d CPU cores detected", numCPU))

	info.Devices = append(info.Devices, Device{
		Type:  CPU,
		ID:    0,
		Name:  fmt.Sprintf("CPU x%d cores", numCPU),
		Score: 10,
		Cores: numCPU,
	})

	if os.Getenv("HAS_GPU") == "1" {
		info.Devices = append(info.Devices, Device{
			Type:  GPU,
			ID:    0,
			Name:  "Generic GPU",
			Score: 50,
		})
		info.Info = append(info.Info, "Generic GPU detected")
	} else {
		info.Warnings = append(info.Warnings, "No GPU detected")
	}

	if os.Getenv("HAS_TPU") == "1" {
		info.Devices = append(info.Devices, Device{
			Type:  TPU,
			ID:    0,
			Name:  "Generic TPU",
			Score: 80,
		})
		info.Info = append(info.Info, "Generic TPU detected")
	} else {
		info.Warnings = append(info.Warnings, "No TPU detected")
	}

	return info
}

// --- Machine Intelligence Helpers ---

func readLoadAvg() float64 {
	data, err := ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return 0.1
	}
	fields := strings.Fields(string(data))
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0.1
	}
	return v
}

func readMemTotal() float64 {
	data, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return 1
	}
	lines := strings.Split(string(data), "\n")
	for _, ln := range lines {
		if strings.HasPrefix(ln, "MemTotal") {
			parts := strings.Fields(ln)
			kb, err := strconv.ParseFloat(parts[1], 64)
			if err == nil {
				return kb / 1024.0 / 1024.0
			}
		}
	}
	return 1
}

// Continuous machine capability score using real system intelligence.
func machineCapabilityScore() float64 {
	cores := float64(runtime.NumCPU())
	load := readLoadAvg()
	mem := readMemTotal()

	// Penalizes overloaded system exponentially.
	loadPenalty := math.Exp(load / cores)

	// GPU/TPU bonuses (no if-else logic for thresholds).
	gpuBonus := map[bool]float64{true: 2.0, false: 0}[os.Getenv("HAS_GPU") == "1"]
	tpuBonus := map[bool]float64{true: 3.0, false: 0}[os.Getenv("HAS_TPU") == "1"]

	score := (cores * 1.2) + (mem * 0.8) + gpuBonus + tpuBonus
	score = score / loadPenalty

	if score < 1 {
		score = 1
	}
	return score
}

// Convert capability score into worker count using smooth scaling.
// No if-else. Pure mathematical scaling.
func RecommendWorkers() int {
	score := machineCapabilityScore()

	workers := int(math.Round(math.Log2(score*4) * 2))

	if workers < 1 {
		workers = 1
	}

	return workers
}

// Return best device (highest score).
func GetBestDevice() (Device, []string, []string, []error) {
	info := DetectDevices()

	if len(info.Devices) == 0 {
		return Device{}, info.Info, info.Warnings, info.Errors
	}

	best := info.Devices[0]
	for _, d := range info.Devices {
		if d.Score > best.Score {
			best = d
		}
	}

	return best, info.Info, info.Warnings, info.Errors
}

func (d Device) String() string {
	return fmt.Sprintf("%s:%d (%s, score=%d)", d.Type, d.ID, d.Name, d.Score)
}
