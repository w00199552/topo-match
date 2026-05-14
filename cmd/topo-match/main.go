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

func main() {
	// CLI mode flags
	logicFile := flag.String("logic", "", "path to logic topology XML file")
	testbedFiles := flag.String("testbed", "", "comma-separated paths to testbed XML files")
	serve := flag.Bool("serve", false, "run as REST API server")
	port := flag.String("port", "8080", "server port (for serve mode)")

	flag.Parse()

	if *serve {
		runServer(*port)
		return
	}

	// CLI mode
	if *logicFile == "" || *testbedFiles == "" {
		fmt.Println("Usage: topo-match --logic <file> --testbed <file1,file2,...>")
		fmt.Println("       topo-match --serve [--port 8080]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	runCLI(*logicFile, *testbedFiles)
}

func runServer(port string) {
	s := server.NewServer()
	fmt.Printf("topo-match server starting on :%s\n", port)
	if err := s.Run(":" + port); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func runCLI(logicFile string, testbedFilesStr string) {
	// Parse logic topology
	logic, err := model.ParseXMLFile(logicFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse logic topology: %v\n", err)
		os.Exit(1)
	}

	// Parse testbed files
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

	// Allocate
	result, err := alloc.Alloc(logic, tbFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "alloc failed: %v\n", err)
		os.Exit(1)
	}

	// Output result as JSON
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal result: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}
