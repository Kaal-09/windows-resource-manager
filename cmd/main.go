package main

import (
	"fmt"
	"resource_manager/internals/processes"
	"runtime"
	"time"
)

func main() {
	previous := make(map[uint32]processes.ResourceUsage)
	logicalCPUs := runtime.NumCPU()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	fmt.Println("Windows Resource Manager")
	fmt.Println("Monitoring processes...")

	for {
		pcs, err := processes.List()
		if err != nil {
			fmt.Println("Process discovery error:", err)
			time.Sleep(time.Second)
			continue
		}
		current := make(map[uint32]processes.ResourceUsage)
		fmt.Print("\033[H\033[2J")
		fmt.Println("WINDOWS RESOURCE MANAGER")
		fmt.Println("Logical CPUs:", logicalCPUs)
		fmt.Println()
		fmt.Printf("%-8s %-24s %10s %10s %12s %12s\n", "PID", "PROCESS", "CPU(1C)", "CPU(SYS)", "WORKING SET", "PRIVATE")
		fmt.Println("-------------------------------------------------------------------------------")
		for _, p := range pcs {
			usage, err := processes.GetResourceUsage(p.PID)
			if err != nil {
				fmt.Printf("%-8d %-24s %10s %10s %12s %12s\n", p.PID, p.Name, "N/A", "N/A", "N/A", "N/A")
				continue
			}
			current[p.PID] = usage
			cpuOneCore := "Baseline"
			cpuMachine := "Baseline"
			if prev, exists := previous[p.PID]; exists {
				oneCore, machine, valid := processes.CalculateCPU(prev, usage, logicalCPUs)
				if valid {
					cpuOneCore = fmt.Sprintf("%.2f%%", oneCore)
					cpuMachine = fmt.Sprintf("%.2f%%", machine)
				}
			}
			workingSetMB := float64(
				usage.WorkingSetBytes,
			) / (1024 * 1024)
			privateMB := float64(
				usage.PrivateBytes,
			) / (1024 * 1024)
			fmt.Printf("%-8d %-24s %10s %10s %10.2f MB %10.2f MB\n", p.PID, p.Name, cpuOneCore, cpuMachine, workingSetMB, privateMB)
		}
		previous = current
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop.")
		<-ticker.C
	}
}
