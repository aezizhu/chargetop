package battery

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
)

type BatteryInfo struct {
	Percent    int
	Status     string
	Remaining  string
	IsCharging bool

	// Real-time (ioreg)
	Temperature float64 // Celsius
	Wattage     int
	Serial      string // ioreg has serial too, but system_profiler is cleaner

	// Static/Health (system_profiler)
	CycleCount  int
	Condition   string // "Normal", "Service Recommended", etc.
	MaxCapacity string // "95%" string from system_profiler
}

var (
	// Cache static info to avoid slow system_profiler calls on every tick
	staticInfoCache BatteryInfo
	lastStaticFetch int64
	mutex           sync.Mutex
)

// GetBatteryInfo combines fast ioreg data with cached accurate system_profiler data
func GetBatteryInfo() (BatteryInfo, error) {
	info := BatteryInfo{
		Status:    "Unknown",
		Remaining: "Calculating...",
	}

	// 1. Fetch Static Info (Condition, Max Cap, Cycles) sparingly
	// In a real app we'd check timestamps, but for now let's just fetch if empty or on a separate ticker.
	// Actually, let's just make a helper we call less often, or doing it here is fine if we accept the overhead?
	// No, system_profiler is slow (can take 1-2s).
	// The `Update` loop in main.go calls this every 2s. We CANNOT call system_profiler every 2s.
	// We must cache it.

	mutex.Lock()
	if staticInfoCache.MaxCapacity == "" {
		// First run, fetch it.
		// We will do this in a goroutine in a real app, but here we might block once.
		// Better: return what we have and trigger a fetch?
		// For simplicity in this script, let's block once.
		fetchStaticInfo(&staticInfoCache)
	}
	info.Condition = staticInfoCache.Condition
	info.MaxCapacity = staticInfoCache.MaxCapacity
	info.CycleCount = staticInfoCache.CycleCount
	// info.Serial = staticInfoCache.Serial // Use ioreg serial if faster? system_profiler serial is reliable.
	mutex.Unlock()

	// 2. Fetch Real-time Info (ioreg) - Fast
	cmd := exec.Command("ioreg", "-r", "-n", "AppleSmartBattery")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return info, err
	}
	output := out.String()

	// ... Parsing Logic ...

	// Percent
	currentCap := getInt(output, `\"CurrentCapacity\"\s*=\s*(\d+)`)
	// maxCapRaw := getInt(output, `\"AppleRawMaxCapacity\"\s*=\s*(\d+)`)
	// Wait, earlier we used `MaxCapacity` which was 100. Let's use `MaxCapacity` for *calculation* of percentage if `CurrentCapacity` is relative to it.
	// Usually CurrentCapacity is mAh.
	// Let's stick to CurrentCapacity / AppleRawMaxCapacity * 100 if MaxCapacity=100.
	// Actually, `MaxCapacity` in ioreg is usually calculation basis.
	maxCapCalc := getInt(output, `\"MaxCapacity\"\s*=\s*(\d+)`)
	if maxCapCalc > 0 {
		info.Percent = (currentCap * 100) / maxCapCalc
	}
	// IsCharging
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
	tr := getInt(output, `\"TimeRemaining\"\s*=\s*(\d+)`)
	if tr < 65535 {
		h := tr / 60
		m := tr % 60
		info.Remaining = fmt.Sprintf("%d:%02d remaining", h, m)
	} else {
		// If charging and 65535, usually means calculating.
		// If charged, it stays empty.
		if info.Status != "Charged" {
			info.Remaining = "Calculating..."
		}
	}

	// Temperature & Watts
	temp := getInt(output, `\"Temperature\"\s*=\s*(\d+)`)
	info.Temperature = float64(temp) / 100.0
	info.Wattage = getInt(output, `\"Watts\"\s*=\s*(\d+)`)
	info.Serial = getString(output, `\"Serial\"\s*=\s*\"([^\"]+)\"`)

	return info, nil
}

func fetchStaticInfo(target *BatteryInfo) {
	// system_profiler SPPowerDataType -json
	cmd := exec.Command("system_profiler", "SPPowerDataType", "-json")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return
	}
	jsonStr := out.String()

	// Parse using Regex for simplicity instead of full struct (Go regex is robust enough for unique keys)
	// "sppower_battery_health" : "Good" or "Normal"
	target.Condition = getString(jsonStr, `\"sppower_battery_health\"\s*:\s*\"([^\"]+)\"`)

	// "sppower_battery_health_maximum_capacity" : "95%"
	target.MaxCapacity = getString(jsonStr, `\"sppower_battery_health_maximum_capacity\"\s*:\s*\"([^\"]+)\"`)

	// "sppower_battery_cycle_count" : 193
	target.CycleCount = getInt(jsonStr, `\"sppower_battery_cycle_count\"\s*:\s*(\d+)`)
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
