package battery

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

type BatteryInfo struct {
	Percent    int
	Status     string
	Remaining  string
	IsCharging bool

	// Advanced Stats
	Temperature float64 // Celsius
	CycleCount  int
	Condition   string // Good/Normal/etc (Inferred from health)
	MaxCapacity int
	Health      string // e.g. "95%" if calculated
	Wattage     int
	Serial      string
}

func GetBatteryInfo() (BatteryInfo, error) {
	info := BatteryInfo{
		Status:    "Unknown",
		Remaining: "Calculating...",
	}

	// 1. Get Basic Info from pmset (it has the best status/remaining logic)
	// We could parse ioreg for everything, but pmset's time remaining is standard.
	// Actually, let's parse ioreg for *everything* to be faster and consistent.
	// ioreg -r -n AppleSmartBattery

	cmd := exec.Command("ioreg", "-r", "-n", "AppleSmartBattery")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return info, err
	}
	output := out.String()

	// Parse Fields

	// State of Charge
	// "CurrentCapacity" = 15
	// "MaxCapacity" = 100
	// Make sure to match specific keys, as MaxCapacity might appear multiple times.
	// Using generic "Key" = Value regex

	currentCap := getInt(output, `\"CurrentCapacity\"\s*=\s*(\d+)`)
	maxCap := getInt(output, `\"MaxCapacity\"\s*=\s*(\d+)`)

	if maxCap > 0 {
		info.Percent = (currentCap * 100) / maxCap
		// Overwrite with pmset check if needed, but this is raw controller data
		// Some people prefer "AppleRawCurrentCapacity" vs "AppleRawMaxCapacity"
	}

	// Use regex to find IsCharging
	// "IsCharging" = Yes
	if getString(output, `\"IsCharging\"\s*=\s*(Yes|No)`) == "Yes" {
		info.IsCharging = true
		info.Status = "Charging"
	} else {
		info.IsCharging = false
		info.Status = "Discharging"
		if getString(output, `\"FullyCharged\"\s*=\s*(Yes)`) == "Yes" {
			info.Status = "Charged"
		}
	}

	// Time Remaining
	// "TimeRemaining" = 177 (minutes)
	tr := getInt(output, `\"TimeRemaining\"\s*=\s*(\d+)`)
	if tr < 65535 {
		h := tr / 60
		m := tr % 60
		info.Remaining = fmt.Sprintf("%d:%02d remaining", h, m)
	} else {
		info.Remaining = "Calculating..." // 65535 often means calculating
		if info.Status == "Charged" {
			info.Remaining = ""
		}
	}

	// Temperature
	// "Temperature" = 3040 (centidegrees)
	temp := getInt(output, `\"Temperature\"\s*=\s*(\d+)`)
	if temp > 0 {
		info.Temperature = float64(temp) / 100.0
	}

	// CycleCount
	// "CycleCount" = 193
	info.CycleCount = getInt(output, `\"CycleCount\"\s*=\s*(\d+)`)

	// Watts
	// "Watts"=60 (inside AdapterDetails)
	info.Wattage = getInt(output, `\"Watts\"=(\d+)`)

	// Serial
	// "Serial" = "F8..."
	info.Serial = getString(output, `\"Serial\"\s*=\s*\"([^\"]+)\"`)

	// Design Cap for Health Calcs
	// "DesignCapacity" = 8579
	designCap := getInt(output, `\"DesignCapacity\"\s*=\s*(\d+)`)
	appleRawMax := getInt(output, `\"AppleRawMaxCapacity\"\s*=\s*(\d+)`)

	if designCap > 0 && appleRawMax > 0 {
		healthPct := (float64(appleRawMax) / float64(designCap)) * 100
		info.Health = fmt.Sprintf("%.0f%%", healthPct)
	}

	info.MaxCapacity = maxCap // This is relative max capacity (wear info is mostly in AppleRawMax vs Design)

	// Condition (Hard to map exactly without system_profiler strings, but we can infer)
	// Or just leave blank if we rely on system_profiler.
	// Let's stick to "Health" % which is more useful.

	return info, nil
}

func getInt(text string, pattern string) int {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		val, _ := strconv.Atoi(matches[1])
		return val
	}
	return 0
}

func getString(text string, pattern string) string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
