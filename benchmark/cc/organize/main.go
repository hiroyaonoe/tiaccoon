package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: main <input_path> <output_path> <prefix>")
		return
	}

	inputPath := os.Args[1]
	outputDir := os.Args[2]
	prefix := os.Args[3]

	files, err := filepath.Glob(filepath.Join(inputPath, prefix, "*", "*.txt"))
	if err != nil {
		fmt.Println("Error finding files:", err)
		return
	}
	if len(files) == 0 {
		fmt.Println("No files found.")
		return
	}

	allRecords := [][]string{{"name", "average", "min", "max", "median", "p90", "p95", "p99"}}

	for _, filePath := range files {
		f, err := os.Open(filePath)
		if err != nil {
			fmt.Println("Error opening file:", err)
			return
		}
		defer f.Close()

		var times []time.Duration
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			nanoSeconds, err := strconv.ParseInt(line, 10, 64)
			if err != nil {
				fmt.Println("Error parsing line:", err)
				return
			}
			times = append(times, time.Duration(nanoSeconds))
		}

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading file:", err)
			return
		}

		total := time.Duration(0)
		for _, t := range times {
			total += t
		}
		average := total / time.Duration(len(times))

		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		min := times[0]
		max := times[len(times)-1]
		median := times[len(times)/2]
		p90 := times[int(float64(len(times))*0.9)]
		p95 := times[int(float64(len(times))*0.95)]
		p99 := times[int(float64(len(times))*0.99)]

		name := filepath.Base(filepath.Dir(filePath))
		allRecords = append(allRecords, []string{
			name,
			strconv.FormatInt(int64(average), 10),
			strconv.FormatInt(int64(min), 10),
			strconv.FormatInt(int64(max), 10),
			strconv.FormatInt(int64(median), 10),
			strconv.FormatInt(int64(p90), 10),
			strconv.FormatInt(int64(p95), 10),
			strconv.FormatInt(int64(p99), 10),
		})
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Println("Error creating output directory:", err)
		return
	}

	outputFile, err := os.Create(filepath.Join(outputDir, prefix+".csv"))
	if err != nil {
		fmt.Println("Error creating output file:", err)
		return
	}
	defer outputFile.Close()

	writer := csv.NewWriter(outputFile)
	defer writer.Flush()

	writer.WriteAll(allRecords)
}
