package patterns

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
)

// CPUPattern is used when changing colors from "cold" (blue) to "hot" (red)
// according to the CPU utilization. The "delay" configuration value expresses
// the amount of time to wait between samples.
type CPUPattern struct {
	BasePattern
}

var _ Pattern = (*CPUPattern)(nil)  // ensures we conform to the Pattern interface
var _ runnable = (*CPUPattern)(nil) // ensures we conform to the runnable interface

func init() {
	register("cpu", &CPUPattern{}, 1*time.Second)
}

func (p *CPUPattern) run() error {
	for {
		previous, err := getCPUStats()
		if err != nil {
			return err
		}

		if p.cancelableSleep() {
			return nil
		}

		current, err := getCPUStats()
		if err != nil {
			return err
		}

		cpuPercentage := float64(current.active-previous.active) / float64(current.total-previous.total)
		i := int(math.Round(float64(len(coldHotColors)-1) * cpuPercentage))
		color := coldHotColors[i]

		err = keyboard.ColorFileHandler(color)
		if err != nil {
			return err
		}
	}
}

type cpuStats struct {
	active int
	total  int
}

func getCPUStats() (*cpuStats, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil, fmt.Errorf("can't open system stats: %w", err)
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("can't read system stats: %w", err)
	}

	parts := strings.Split(line, " ")

	// name, _ := strconv.Atoi(parts[0])
	user, _ := strconv.Atoi(parts[1])
	nice, _ := strconv.Atoi(parts[2])
	system, _ := strconv.Atoi(parts[3])
	idle, _ := strconv.Atoi(parts[4])
	iowait, _ := strconv.Atoi(parts[5])
	// irq, _ := strconv.Atoi(parts[6])
	softirq, _ := strconv.Atoi(parts[7])
	steal, _ := strconv.Atoi(parts[8])
	// guest, _ := strconv.Atoi(parts[9])

	stats := &cpuStats{active: user + system + nice + softirq + steal}
	stats.total = stats.active + idle + iowait

	return stats, nil
}
