package main

import (
	"fmt"
	"os"
	"resource_manager/internals/processes"
	"strconv"
)

func main() {

	if len(os.Args) > 1 {
		pid, err := strconv.ParseUint(os.Args[1], 10, 32)
		if err != nil {
			fmt.Println("Invalid PID:", err)
			return
		}
		p, err := processes.GetByPID(uint32(pid))
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Println("Process found:")
		fmt.Printf("PID:        %d\n", p.PID)
		fmt.Printf("Parent PID: %d\n", p.ParentPID)
		fmt.Printf("Name:       %s\n", p.Name)
		return
	}

	processes, err := processes.List()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("%-10s %-15s %-25s %-25s\n", "PID", "PARENT PID", "PROCESS NAME", "OWNER")
	fmt.Println("-------------------------------------------------------")

	for _, p := range processes {
		owner := p.Domain + "\\" + p.Owner
		if p.OwnerErr != "" {
			owner = "Access unavailable"
		}
		fmt.Printf("%-10d %-15d %-25s %-25s\n", p.PID, p.ParentPID, p.Name, owner)
	}
	fmt.Printf("\nTotal processes: %d\n", len(processes))
}
