package main

import (
	"fmt"
	"runtime"
)

func burn() {
	var x uint64 = 1
	for {
		x = x*6364136223846793005 + 1442695040888963407
		if x == 0 {
			runtime.Gosched()
		}
	}
}

func main() {
	n := runtime.NumCPU() - 5
	fmt.Printf("Burning %d logical cores\n", n)
	for i := 0; i < n; i++ {
		go burn()
	}
	select {}
}
