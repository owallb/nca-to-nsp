package main

import (
	"flag"
	"fmt"
	"nca-to-nsp/pkg/nsp"
	"os"
)

// Application information
const (
	appName        = "nca-to-nsp"
	appVersion     = "1.0.0"
	appDescription = "A utility to package Nintendo Content Archive (NCA) " +
		"files into a Nintendo Submission Package (NSP)."
)

// exitWithError prints an error message to stderr and exits the program with
// code 1.
// Accepts either a single value or a format string with arguments.
func exitWithError(arg any, args ...any) {
	var msg string

	if len(args) == 0 {
		msg = fmt.Sprintf("%v", arg)
	} else {
		msg = fmt.Sprintf(arg.(string), args...)
	}

	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	os.Exit(1)
}

// printDescription displays application description to stdout
func printDescription() {
	fmt.Printf("%s v%s - %s\n\n", appName, appVersion, appDescription)
}

// Display the application usage information and exits.
func printUsage() {
	fmt.Println("Usage:")
	fmt.Printf(
		"  %s -o <output.nsp> [options] file1.nca [file2.nca ...]\n\n",
		os.Args[0],
	)
	fmt.Println("Options:")
	flag.PrintDefaults()
}

// printVersion displays the application version information to stdout
func printVersion() {
	fmt.Printf("%s v%s\n", appName, appVersion)
}

func main() {
	helpFlag := flag.Bool("h", false, "Display help information")
	versionFlag := flag.Bool("v", false, "Display version information")
	outputName := flag.String("o", "out.nsp", "NSP output file name")
	bufferSize := flag.Int(
		"buffer",
		nsp.DefaultBufferSize,
		"Buffer size for file copying operations",
	)
	showProgress := flag.Bool(
		"progress",
		false,
		"Show progress bar during NSP creation",
	)
	flag.Parse()

	if *helpFlag {
		printDescription()
		printUsage()
		os.Exit(0)
	}

	if *versionFlag {
		printVersion()
		os.Exit(0)
	}

	ncaFiles := flag.Args()

	if len(ncaFiles) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no NCA files specified")
		printUsage()
		os.Exit(1)
	}

	builder := nsp.Builder{
		OutputPath:              *outputName,
		BufferSize:              *bufferSize,
		ShowProgress:            *showProgress,
		ProgressUpdateFrequency: 100,
	}

	if err := builder.AddFiles(ncaFiles); err != nil {
		exitWithError("Failed to add files to NSP builder: %v", err)
	}

	if err := builder.Build(); err != nil {
		exitWithError("Failed to build NSP file: %v", err)
	}

	fmt.Printf("Successfully built NSP file: %s\n", *outputName)
}
