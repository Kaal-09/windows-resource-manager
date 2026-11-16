package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"resource_manager/internals/enforcement"
	"resource_manager/internals/processes"
)

const usageText = `Windows Resource Manager

Usage:
  resource_manager [monitor]            watch process CPU and memory usage
  resource_manager limit <PID> <CPU%>   apply a CPU hard cap (1-100) until Ctrl+C
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := "monitor"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "monitor":
		return runMonitor(ctx)
	case "limit":
		return runLimit(ctx, args[1:])
	case "help", "-h", "--help":
		fmt.Print(usageText)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usageText)
		return 2
	}
}

func runLimit(ctx context.Context, args []string) int {
	pid, cpuLimit, err := parseLimitArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}

	job, err := enforcement.ApplyCPULimit(pid, cpuLimit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to apply CPU limit:", err)
		return 1
	}

	fmt.Printf("Successfully applied CPU hard cap!\nPID: %d\nCPU Limit: %d%%\n", pid, cpuLimit)
	fmt.Println("Press Ctrl+C to remove the CPU restriction.")

	<-ctx.Done() // block until a signal cancels the context

	fmt.Println("\nDisabling CPU restriction...")
	exitCode := 0

	if err := job.DisableCPULimit(); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to disable CPU restriction:", err)
		exitCode = 1
	} else {
		fmt.Println("CPU restriction disabled.")
	}

	if err := job.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to close Job Object:", err)
		exitCode = 1
	}

	fmt.Println("Resource manager stopped.")
	return exitCode
}

func parseLimitArgs(args []string) (pid, cpuLimit uint32, err error) {
	if len(args) != 2 {
		return 0, 0, errors.New("limit requires <PID> and <CPU%>")
	}

	pidValue, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil || pidValue == 0 {
		return 0, 0, errors.New("invalid PID")
	}

	cpuValue, err := strconv.ParseUint(args[1], 10, 32)
	if err != nil || cpuValue < 1 || cpuValue > 100 {
		return 0, 0, errors.New("CPU limit must be between 1 and 100")
	}

	return uint32(pidValue), uint32(cpuValue), nil
}

func runMonitor(ctx context.Context) int {
	logicalCPUs := runtime.NumCPU()
	previous := make(map[uint32]processes.ResourceUsage)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		discoverer := processes.NewDiscoverer()
		current, frame, err := renderFrame(discoverer, previous, logicalCPUs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Process discovery error:", err)
		} else {
			fmt.Print(frame) // one write per frame avoids flicker
			previous = current
		}

		select {
		case <-ctx.Done():
			fmt.Println("\nMonitor stopped.")
			return 0
		case <-ticker.C:
		}
	}
}

func renderFrame(discoverer processes.ProcessDiscoverer, previous map[uint32]processes.ResourceUsage, logicalCPUs int) (map[uint32]processes.ResourceUsage, string, error) {
	pcs, err := discoverer.List()
	if err != nil {
		return nil, "", err
	}

	const rowFmt = "%-8s %-24s %10s %10s %12s %12s\n"
	var buf bytes.Buffer

	buf.WriteString("\033[H\033[2J") // clear screen
	buf.WriteString("WINDOWS RESOURCE MANAGER\n")
	fmt.Fprintf(&buf, "Logical CPUs: %d\n\n", logicalCPUs)
	fmt.Fprintf(&buf, rowFmt, "PID", "PROCESS", "CPU(1C)", "CPU(SYS)", "WORKING SET", "PRIVATE")
	buf.WriteString("-------------------------------------------------------------------------------\n")

	current := make(map[uint32]processes.ResourceUsage, len(pcs))

	for _, p := range pcs {
		pid := strconv.FormatUint(uint64(p.PID), 10)

		usage, err := discoverer.Usage(p.PID)
		if err != nil {
			fmt.Fprintf(&buf, rowFmt, pid, p.Name, "N/A", "N/A", "N/A", "N/A")
			continue
		}
		current[p.PID] = *usage

		cpuOneCore, cpuMachine := "Baseline", "Baseline"
		if prev, ok := previous[p.PID]; ok {
			if oneCore, machine, valid := processes.CalculateCPU(prev, *usage, logicalCPUs); valid {
				cpuOneCore = fmt.Sprintf("%.2f%%", oneCore)
				cpuMachine = fmt.Sprintf("%.2f%%", machine)
			}
		}

		fmt.Fprintf(&buf, rowFmt, pid, p.Name, cpuOneCore, cpuMachine, formatMB(usage.WorkingSetBytes), formatMB(usage.PrivateBytes))
	}

	buf.WriteString("\nPress Ctrl+C to stop.\n")
	return current, buf.String(), nil
}

func formatMB[T ~uint64 | ~int64 | ~uint | ~uintptr](bytes T) string {
	return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
}
