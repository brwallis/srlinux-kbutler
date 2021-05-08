package cpu

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Core represents a physical machine core (containing two threads) - we always assume HT
type Core struct {
	CPUA int
	CPUB int
}

// NewCore creates a new Core
func NewCore(cpuA int, cpuB int) Core {
	return Core{CPUA: cpuA, CPUB: cpuB}
}

// GetChildren gets the children of a core
func (c *Core) GetChildren() []int {
	var cpuList []int
	cpuList = append(cpuList, c.CPUA)
	cpuList = append(cpuList, c.CPUB)
	return cpuList
}

// cpuSetDelta takes two slices of ints, doing a delta and returning the difference
func cpuSetDelta(cpuAll, cpuIsolated []int) []int {
	mb := make(map[int]struct{}, len(cpuIsolated))
	for _, x := range cpuIsolated {
		mb[x] = struct{}{}
	}
	var diff []int
	for _, x := range cpuAll {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

//FindCoreThreadPairs, given a CPU set, in the form of a []int, try to figure out the core layout TODO (@bwallis) this doesn't deal with us not being passed both threads of a core yet
func FindCoreThreadPairs(cpuList []int) []Core {
	var coreList []Core
	c := make(map[int]Core)
	for _, cpu := range cpuList {
		// Process the CPU, first checking for its pair
		if _, ok := c[cpu]; !ok {
			// We could not find a core ID with the CPU ID, check to see if the sibling has one yet
			cpuSibling := Sibling(cpu)
			if _, ok := c[cpuSibling]; !ok {
				var currCore Core
				// No reference to the sibling either, add the core, with the lower index being the key (since it aligns with the core ID in most cases)
				if cpu < cpuSibling {
					currCore = NewCore(cpu, cpuSibling)
				} else {
					currCore = NewCore(cpuSibling, cpu)
				}
				fmt.Printf("Adding core %d to cpuIn\n", cpu)
				coreList = append(coreList, currCore)
				c[cpu] = currCore
			}
		}
	}
	return coreList
}

// Sibling returns the sibling of a cpu
func Sibling(cpuID int) int {
	var cpuSibling int
	sysFSPath := "/sys/bus/cpu/devices/cpu" + strconv.Itoa(cpuID) + "/"
	topologyPath := "topology/core_cpus_list"
	cpuSetData, err := ioutil.ReadFile(sysFSPath + topologyPath)
	if err != nil {
		log.Fatal(err)
	}
	cpuSet := string(cpuSetData)
	cpuSet = cpuSet[:len(cpuSet)-1]
	// Now split the string, and return the value that is not equal to the CPU passed in
	cpuSetStrings := strings.Split(cpuSet, ",")
	for _, cpuSiblingFromFile := range cpuSetStrings {
		cpuInt, err := strconv.ParseInt(cpuSiblingFromFile, 10, 32)
		if err != nil {
			log.Fatal(err)
		}
		if int(cpuInt) == cpuID {
			continue
		}
		cpuSibling = int(cpuInt)
	}
	return cpuSibling
}

// TotalCPUs returns a single int containing the number of CPUs available to the go runtime
func TotalCPUs() int {
	return runtime.NumCPU()
}

// parseCPUSet takes a CPU set (in format "n-m,o-p,..."), and returns a slice with a CPU per index
// using terminology cpuset = n-m,o-p, cpurange = n-m, cpu = n
func ParseCPUSet(cpuSet string) []int {
	var cpuSlice []int
	cpuSetStrings := strings.Split(cpuSet, ",")
	// Convert cpuSetStrings to cpuSet
	for _, cpuRange := range cpuSetStrings {
		// Check if this is a range or an individual CPU
		if strings.Contains(cpuRange, "-") {
			// Split the string again
			cpuRangeStrings := strings.Split(cpuRange, "-")
			cpuRangeStart, err := strconv.ParseInt(cpuRangeStrings[0], 10, 32)
			if err != nil {
				log.Fatal(err)
			}
			cpuRangeEnd, err := strconv.ParseInt(cpuRangeStrings[1], 10, 32)
			if err != nil {
				log.Fatal(err)
			}
			for cpu := int(cpuRangeStart); cpu <= int(cpuRangeEnd); cpu++ {
				cpuSlice = append(cpuSlice, cpu)
			}
			continue
		}
		cpuInt, err := strconv.ParseInt(cpuRange, 10, 32)
		if err != nil {
			log.Fatal(err)
		}
		cpuSlice = append(cpuSlice, int(cpuInt))
	}
	return cpuSlice
}

// CPUSetPID returns the CPU's in use by a given PID, assuming taskset is available
func CPUSetPID(pid int) []int {
	taskSetExecutable, _ := exec.LookPath("taskset")
	out, err := exec.Command(taskSetExecutable, "-pc", strconv.Itoa(pid)).Output()
	if err != nil {
		log.Fatal(err)
	}
	words := strings.Fields(string(out))
	return ParseCPUSet(words[len(words)-1])
}

// EntryCPUSet gets the cpuset of pid 1
// on baremetal this will be the cpuset of the init process
// on a container this will be the cpuset of the entrypoint process
func EntryCPUSet() []int {
	pid := 1
	return CPUSetPID(pid)
}

//func main() {
//	fmt.Println(TotalCPUs())
//	fmt.Println(EntryCPUSet())
//	fmt.Printf("Cores: %v", findCoreThreadPairs(EntryCPUSet()))
//}
