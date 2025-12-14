package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	var prefix, inputPath, outputPath string
	flag.StringVar(&prefix, "prefix", "", "Prefix for input files")
	flag.StringVar(&inputPath, "input", "", "Input directory path")
	flag.StringVar(&outputPath, "output", "", "Output directory path")
	flag.Parse()

	if prefix == "" || inputPath == "" || outputPath == "" {
		fmt.Println("Usage: main -prefix <PREFIX> -input <INPUT> -output <OUTPUT>")
		return
	}

	types := map[string]map[string]int{
		"STREAM": {
			"throughput":   4,
			"cpu-total":    5,
			"cpu-per-byte": 7,
		},
		"RR": {
			"latency":     5,
			"transaction": 5,
		},
	}

	for name, kinds := range types {

		files, err := filepath.Glob(filepath.Join(inputPath, prefix, name, "*/*/*/*.log"))
		if err != nil {
			fmt.Println("Error finding files:", err)
			return
		}

		// STREAM
		// Recv   Send    Send                          Utilization       Service Demand
		// Socket Socket  Message  Elapsed              Send     Recv     Send    Recv
		// Size   Size    Size     Time     Throughput  local    remote   local   remote
		// bytes  bytes   bytes    secs.    10^6bits/s  % S      % S      us/KB   us/KB

		//  32768  32768      1    10.00         7.47   12.15    12.15    2133.446  2132.820

		// RR
		// Socket Size   Request Resp.  Elapsed Trans.   CPU    CPU    S.dem   S.dem
		// Send   Recv   Size    Size   Time    Rate     local  remote local   remote
		// bytes  bytes  bytes   bytes  secs.   per sec  % S    % S    us/Tr   us/Tr

		// 425984 425984 1       1      10.00   308239.39  14.00  14.01  7.268   7.270
		// 425984 425984

		data := make(map[string]map[string]map[string]map[string]string)
		for _, file := range files {
			parts := strings.Split(file, string(os.PathSeparator))
			if len(parts) < 7 {
				continue
			}
			bufSize := parts[4]
			key := parts[5]
			size := parts[6]

			for kind, index := range kinds {
				var value string
				var err error
				switch name {
				case "STREAM":
					value, err = extractSTREAM(file, index)
				case "RR":
					value, err = extractRR(file, index, kind)
				}
				if err != nil {
					fmt.Println("Error reading file:", err)
					continue
				}
				if _, exists := data[kind]; !exists {
					data[kind] = make(map[string]map[string]map[string]string)
				}

				if _, exists := data[kind][bufSize]; !exists {
					data[kind][bufSize] = make(map[string]map[string]string)
				}

				if _, exists := data[kind][bufSize][key]; !exists {
					data[kind][bufSize][key] = make(map[string]string)
				}
				data[kind][bufSize][key][size] = value
			}
		}

		for kind := range data {
			for bufSize, keys := range data[kind] {
				outputFile := filepath.Join(outputPath, prefix, name, kind, fmt.Sprintf("%s.csv", bufSize))
				writeCSV(outputFile, keys)
			}
		}
	}
}

func extractSTREAM(filePath string, index int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		search := []string{"Recv", "Send", "Utilization", "Service", "Demand"}
		ok := true
		for _, s := range search {
			if !strings.Contains(line, s) {
				ok = false
				break
			}
		}
		if ok {
			for range 5 {
				if !scanner.Scan() {
					return "", fmt.Errorf("Failed to scan: %s", filePath)
				}
			}
			line = scanner.Text()
			args := strings.Fields(line)
			if len(args) == 9 {
				return args[index], nil
			}
			return "", fmt.Errorf("Failed to get throughput: %s", filePath)
		}
	}
	return "", fmt.Errorf("Throughput not found in file: %s", filePath)
}

func extractRR(filePath string, index int, kind string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		search := []string{"Socket", "Size", "Request", "Elapsed", "Trans"} // RR
		ok := true
		for _, s := range search {
			if !strings.Contains(line, s) {
				ok = false
				break
			}
		}
		if ok {
			for range 4 { // RR
				if !scanner.Scan() {
					return "", fmt.Errorf("Failed to scan: %s", filePath)
				}
			}
			line = scanner.Text()
			args := strings.Fields(line)
			if len(args) == 10 { // RR
				// RR
				value := args[index]
				if kind == "latency" {
					trans, err := strconv.ParseFloat(value, 64)
					if err != nil {
						return "", fmt.Errorf("Failed to parse transaction for latency: %s", filePath)
					}
					latency := 1000000000 / trans // nano seconds
					return fmt.Sprintf("%f", latency), nil
				} else if kind == "transaction" {
					return value, nil
				}

				return "", fmt.Errorf("Failed to get %s: %s", kind, filePath) // RR
			}
			return "", fmt.Errorf("Failed to get latency: %s", filePath) // RR
		}
	}

	return "", fmt.Errorf("Latency not found in file: %s", filePath) // RR
}

func writeCSV(outputPath string, data map[string]map[string]string) {
	if err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm); err != nil {
		fmt.Println("Error creating directories:", err)
		return
	}

	file, err := os.Create(outputPath)
	if err != nil {
		fmt.Println("Error creating CSV file:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{""}
	sizes := make([]string, 0, len(data))
	for size := range data {
		sizes = append(sizes, size)
	}
	sort.Strings(sizes)

	for _, size := range sizes {
		for s := range data[size] {
			header = append(header, s)
		}
		break
	}
	sort.Slice(header, func(i, j int) bool {
		headerI, _ := strconv.Atoi(header[i])
		headerJ, _ := strconv.Atoi(header[j])
		return headerI < headerJ
	})
	writer.Write(header)

	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		row := []string{key}
		for _, size := range header[1:] {
			row = append(row, data[key][size])
		}
		writer.Write(row)
	}
}
