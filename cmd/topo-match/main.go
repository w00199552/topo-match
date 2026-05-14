package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/w00199552/topo-match/internal/allocator"
	"github.com/w00199552/topo-match/internal/model"
	"github.com/w00199552/topo-match/internal/server"
)

var version = "0.2.0"

func main() {
	logicFile := flag.String("logic", "", "path to logic topology XML file")
	testbedFiles := flag.String("testbed", "", "comma-separated paths to testbed XML files")
	freeFile := flag.String("free", "", "path to JSON file with alloc result to free (requires --testbed)")
	serve := flag.Bool("serve", false, "run as REST API server")
	port := flag.String("port", "8080", "server port (for serve mode)")
	showVersion := flag.Bool("version", false, "print version")

	flag.Parse()

	if *showVersion {
		fmt.Printf("topo-match %s\n", version)
		return
	}

	if *serve {
		runServer(*port)
		return
	}

	if *freeFile != "" {
		runFree(*freeFile, *testbedFiles)
		return
	}

	if *logicFile == "" || *testbedFiles == "" {
		fmt.Println("Usage:")
		fmt.Println("  topo-match --logic <file> --testbed <file1,file2,...>   Allocate environment")
		fmt.Println("  topo-match --free <result.json> --testbed <files>       Free environment")
		fmt.Println("  topo-match --serve [--port 8080]                         Run REST API server")
		fmt.Println("  topo-match --version                                     Print version")
		flag.PrintDefaults()
		os.Exit(1)
	}

	runAlloc(*logicFile, *testbedFiles)
}

func runServer(port string) {
	s := server.NewServer()
	fmt.Printf("topo-match server starting on :%s\n", port)
	if err := s.Run(":" + port); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func runAlloc(logicFile string, testbedFilesStr string) {
	logic, err := model.ParseXMLFile(logicFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse logic topology: %v\n", err)
		os.Exit(1)
	}

	tbFiles := strings.Split(testbedFilesStr, ",")
	alloc := allocator.NewAllocator()

	for _, f := range tbFiles {
		f = strings.TrimSpace(f)
		topo, err := model.ParseXMLFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse testbed %s: %v\n", f, err)
			os.Exit(1)
		}
		alloc.AddTestbed(f, topo)
	}

	result, err := alloc.Alloc(logic, tbFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "alloc failed: %v\n", err)
		os.Exit(1)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal result: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}

func runFree(resultFile string, testbedFilesStr string) {
	data, err := os.ReadFile(resultFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read result file: %v\n", err)
		os.Exit(1)
	}

	var allocResult allocator.AllocResult
	if err := json.Unmarshal(data, &allocResult); err != nil {
		fmt.Fprintf(os.Stderr, "parse result JSON: %v\n", err)
		os.Exit(1)
	}

	// Load testbeds to restore state
	if testbedFilesStr == "" {
		fmt.Fprintf(os.Stderr, "Error: --testbed is required with --free to reload testbed state\n")
		os.Exit(1)
	}

	tbFiles := strings.Split(testbedFilesStr, ",")
	alloc := allocator.NewAllocator()

	for _, f := range tbFiles {
		f = strings.TrimSpace(f)
		topo, err := model.ParseXMLFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse testbed %s: %v\n", f, err)
			os.Exit(1)
		}
		alloc.AddTestbed(f, topo)
	}

	if err := alloc.Free(&allocResult); err != nil {
		fmt.Fprintf(os.Stderr, "free failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully freed allocation on testbed: %s\n", allocResult.TestbedName)
	for uuid, node := range allocResult.Nodes {
		fmt.Printf("  %s (%s) -> idle\n", uuid, node.ObjName)
	}
}
